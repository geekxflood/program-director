"""LangChain agent for generating themed playlists."""

import json
from typing import Any

from langchain_core.messages import HumanMessage, SystemMessage
from langchain_core.output_parsers import JsonOutputParser
from langchain_ollama import ChatOllama
from pydantic import BaseModel, Field

from .config import AgentConfig, ThemeConfig
from .scanner import MediaLibrary
from .tunarr_client import TunarrClient


class PlaylistSuggestion(BaseModel):
    """Suggested playlist from the LLM."""

    theme_name: str = Field(description="Name of the theme")
    collection_name: str = Field(description="Name for the Tunarr custom show")
    selected_movies: list[str] = Field(
        default_factory=list, description="List of selected movie titles"
    )
    selected_shows: list[str] = Field(
        default_factory=list, description="List of selected TV show titles"
    )
    selected_anime: list[str] = Field(
        default_factory=list, description="List of selected anime titles"
    )
    reasoning: str = Field(
        default="", description="Why these titles were selected for the theme"
    )
    estimated_runtime: int = Field(
        default=0, description="Estimated runtime in minutes"
    )


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
            num_ctx=8192,  # Increase context window for media library summary
        )
        self.library = MediaLibrary(
            radarr_url=config.radarr.url,
            radarr_api_key=config.radarr.api_key,
            sonarr_url=config.sonarr.url,
            sonarr_api_key=config.sonarr.api_key,
        )
        self.tunarr = TunarrClient(config.tunarr.url)
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

    def get_selected_titles(self, suggestion: PlaylistSuggestion) -> list[str]:
        """Get all selected titles from the suggestion."""
        titles = []

        # Add movies (remove year suffix for cleaner matching)
        for title in suggestion.selected_movies:
            clean_title = title.split(" (")[0] if " (" in title else title
            titles.append(clean_title)

        # Add shows
        titles.extend(suggestion.selected_shows)

        # Add anime
        titles.extend(suggestion.selected_anime)

        return titles

    def apply_playlist(self, suggestion: PlaylistSuggestion) -> bool:
        """Apply the playlist suggestion by creating a custom show in Tunarr."""
        try:
            titles = self.get_selected_titles(suggestion)
            if not titles:
                print("No titles selected for custom show")
                return False

            # Use collection name as custom show name
            show_name = suggestion.collection_name

            # Check if custom show already exists
            existing = self.tunarr.get_custom_shows()
            existing_names = {c.name: c.id for c in existing}

            # Tunarr custom shows store programs, but we create empty shows
            # that can be populated via Tunarr's UI or API with actual media
            # For now, we create/update the custom show with the name
            # The actual program content would need to be added via Tunarr's
            # media library integration

            if show_name in existing_names:
                # Update existing custom show
                result = self.tunarr.update_custom_show(
                    show_id=existing_names[show_name],
                    name=show_name,
                    programs=[],  # Programs would be added via Tunarr UI
                )
            else:
                # Create new custom show
                result = self.tunarr.create_custom_show(
                    name=show_name,
                    programs=[],  # Programs would be added via Tunarr UI
                )

            return result is not None

        except Exception as e:
            print(f"Error applying playlist: {e}")
            return False

    def generate_and_apply(self, theme: ThemeConfig) -> dict[str, Any]:
        """Generate a playlist and apply it to Tunarr."""
        result: dict[str, Any] = {
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
            print(f"  Custom show name: {suggestion.collection_name}")

            result["applied"] = self.apply_playlist(suggestion)
            result["success"] = result["applied"]

            if result["applied"]:
                print("  Successfully created/updated custom show in Tunarr")
            else:
                print("  Failed to apply to Tunarr")
        else:
            print("  Failed to generate playlist suggestion")

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
        self.tunarr.close()
        self.library.close()
