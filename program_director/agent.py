"""LangChain agent for generating themed playlists."""

import json
from typing import Any

from langchain_core.messages import HumanMessage, SystemMessage
from langchain_core.output_parsers import JsonOutputParser
from langchain_ollama import ChatOllama
from pydantic import BaseModel, Field

from .config import AgentConfig, ThemeConfig
from .ersatztv_client import ErsatzTVClient
from .scanner import MediaLibrary


class PlaylistSuggestion(BaseModel):
    """Suggested playlist from the LLM."""

    theme_name: str = Field(description="Name of the theme")
    collection_name: str = Field(description="Name for the ErsatzTV smart collection")
    selected_movies: list[str] = Field(default_factory=list, description="List of selected movie titles")
    selected_shows: list[str] = Field(default_factory=list, description="List of selected TV show titles")
    selected_anime: list[str] = Field(default_factory=list, description="List of selected anime titles")
    reasoning: str = Field(default="", description="Why these titles were selected for the theme")
    estimated_runtime: int = Field(default=0, description="Estimated runtime in minutes")


class PlaylistAgent:
    """AI agent for generating themed playlists using LangChain and Ollama."""

    SYSTEM_PROMPT = """You are a media curator AI that creates themed playlists for a personal TV channel.

Your task is to select media titles that fit a specific theme for an evening viewing session.
You have access to a media library with movies, TV shows, and anime - with full metadata including genres, ratings, and runtimes.

When creating a playlist:
1. Select titles that strongly match the theme based on genres and descriptions
2. Prioritize highly-rated content (7.0+ ratings)
3. Consider variety - mix movies and TV shows/anime if appropriate
4. Aim for the target duration by considering runtime
5. Prefer well-known, quality titles
6. Create a cohesive viewing experience

You MUST respond with valid JSON matching this exact schema:
{{
    "theme_name": "the theme name",
    "collection_name": "Short collection name (max 50 chars)",
    "selected_movies": ["Movie Title 1 (Year)", "Movie Title 2 (Year)", ...],
    "selected_shows": ["Show Title 1", "Show Title 2", ...],
    "selected_anime": ["Anime Title 1", "Anime Title 2", ...],
    "reasoning": "Brief explanation of your selections and why they fit the theme",
    "estimated_runtime": 180
}}

IMPORTANT:
- Only include titles that EXACTLY match titles in the provided media library
- Include the year in parentheses for movies
- Leave arrays empty if no matching content for that category
- Ensure estimated_runtime is calculated from the selected content
- Collection name MUST be 50 characters or less"""

    def __init__(self, config: AgentConfig):
        self.config = config
        self.llm = ChatOllama(
            base_url=config.ollama.url,
            model=config.ollama.model,
            temperature=0.7,
            num_ctx=8192,  # Increase context window to fit media library summary
        )
        self.library = MediaLibrary(
            radarr_url=config.radarr.url,
            radarr_api_key=config.radarr.api_key,
            sonarr_url=config.sonarr.url,
            sonarr_api_key=config.sonarr.api_key,
        )
        self.ersatztv = ErsatzTVClient(config.ersatztv.url)
        self.output_parser = JsonOutputParser(pydantic_object=PlaylistSuggestion)

    def generate_playlist(self, theme: ThemeConfig) -> PlaylistSuggestion | None:
        """Generate a themed playlist using the LLM."""
        # Get media library summary with rich metadata
        media_summary = self.library.get_media_summary()

        # Also get genre statistics for context
        genre_stats = self.library.get_genre_stats()
        top_genres = ", ".join([f"{g}: {c}" for g, c in list(genre_stats.items())[:10]])

        # Create the prompt
        user_prompt = f"""Create a playlist for the theme: "{theme.name}"
Theme description: {theme.description}
Theme keywords: {', '.join(theme.keywords) if theme.keywords else 'none specified'}
Target duration: {theme.duration} minutes

Available genres in library: {top_genres}

{media_summary}

Select appropriate titles from the library above that best match the theme.
Calculate runtime based on movie runtimes and estimated episode lengths.
Respond with valid JSON only."""

        messages = [
            SystemMessage(content=self.SYSTEM_PROMPT),
            HumanMessage(content=user_prompt),
        ]

        try:
            response = self.llm.invoke(messages)
            content = response.content

            # Parse JSON from response
            if isinstance(content, str):
                # Try to extract JSON from the response
                json_start = content.find("{")
                json_end = content.rfind("}") + 1
                if json_start != -1 and json_end > json_start:
                    json_str = content[json_start:json_end]
                    data = json.loads(json_str)
                    return PlaylistSuggestion(**data)

        except Exception as e:
            print(f"Error generating playlist: {e}")
            return None

        return None

    def create_smart_collection_query(self, suggestion: PlaylistSuggestion) -> str:
        """Create an ErsatzTV smart collection query for the given titles."""
        # ErsatzTV uses a specific query format for smart collections
        # This creates an OR query matching any of the selected titles
        conditions = []

        # Add movies
        for title in suggestion.selected_movies:
            # Remove year from title if present for cleaner matching
            clean_title = title.split(" (")[0] if " (" in title else title
            escaped_title = clean_title.replace('"', '\\"')
            conditions.append(f'title contains "{escaped_title}"')

        # Add shows
        for title in suggestion.selected_shows:
            escaped_title = title.replace('"', '\\"')
            conditions.append(f'title contains "{escaped_title}"')

        # Add anime
        for title in suggestion.selected_anime:
            escaped_title = title.replace('"', '\\"')
            conditions.append(f'title contains "{escaped_title}"')

        return " OR ".join(conditions) if conditions else ""

    def apply_playlist(self, suggestion: PlaylistSuggestion) -> bool:
        """Apply the playlist suggestion by creating a smart collection in ErsatzTV."""
        try:
            query = self.create_smart_collection_query(suggestion)
            if not query:
                print("No titles selected for collection")
                return False

            # Truncate collection name to 50 chars (ErsatzTV limit)
            collection_name = suggestion.collection_name[:50]

            # Check if collection already exists
            existing = self.ersatztv.get_smart_collections()
            existing_names = {c.name: c.id for c in existing}

            if collection_name in existing_names:
                # Update existing collection
                result = self.ersatztv.update_smart_collection(
                    collection_id=existing_names[collection_name],
                    name=collection_name,
                    query=query,
                )
            else:
                # Create new collection
                result = self.ersatztv.create_smart_collection(
                    name=collection_name,
                    query=query,
                )

            return result is not None

        except Exception as e:
            print(f"Error applying playlist: {e}")
            return False

    def generate_and_apply(self, theme: ThemeConfig) -> dict[str, Any]:
        """Generate a playlist and apply it to ErsatzTV."""
        result = {
            "theme": theme.name,
            "success": False,
            "suggestion": None,
            "applied": False,
        }

        print(f"Generating playlist for theme: {theme.name}")
        suggestion = self.generate_playlist(theme)

        if suggestion:
            result["suggestion"] = suggestion.model_dump()
            total_selections = (
                len(suggestion.selected_movies)
                + len(suggestion.selected_shows)
                + len(suggestion.selected_anime)
            )
            print(f"  Selected {total_selections} titles")
            print(f"  Collection name: {suggestion.collection_name}")

            result["applied"] = self.apply_playlist(suggestion)
            result["success"] = result["applied"]

            if result["applied"]:
                print(f"  Successfully created/updated collection in ErsatzTV")
            else:
                print(f"  Failed to apply to ErsatzTV")
        else:
            print(f"  Failed to generate playlist suggestion")

        return result

    def generate_all_themes(self) -> list[dict[str, Any]]:
        """Generate playlists for all configured themes."""
        results = []
        for theme in self.config.themes:
            result = self.generate_and_apply(theme)
            results.append(result)
        return results

    def close(self) -> None:
        """Clean up resources."""
        self.ersatztv.close()
        self.library.close()
