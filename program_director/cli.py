"""Command-line interface for Program Director."""

from pathlib import Path

import typer
from rich.console import Console
from rich.table import Table

from .agent import PlaylistAgent
from .config import load_config

app = typer.Typer(
    name="program-director",
    help="AI-powered TV channel programmer for Tunarr using Radarr/Sonarr metadata",
)
console = Console()


@app.command()
def generate(
    theme: str = typer.Option(None, "--theme", "-t", help="Theme name to generate"),
    all_themes: bool = typer.Option(False, "--all-themes", "-a", help="Generate all themes"),
    config_file: Path = typer.Option(
        Path("/app/config/config.yaml"),
        "--config",
        "-c",
        help="Path to configuration file",
    ),
    dry_run: bool = typer.Option(False, "--dry-run", "-n", help="Don't apply to Tunarr"),
) -> None:
    """Generate themed playlists using AI."""
    config = load_config(config_file)

    if not config.themes:
        console.print("[red]No themes configured![/red]")
        raise typer.Exit(1)

    if not config.radarr.api_key or not config.sonarr.api_key:
        console.print("[red]Radarr and Sonarr API keys are required![/red]")
        console.print("Set RADARR_API_KEY and SONARR_API_KEY environment variables")
        raise typer.Exit(1)

    agent = PlaylistAgent(config)

    try:
        if all_themes:
            console.print("[bold]Generating playlists for all themes...[/bold]\n")

            if dry_run:
                for theme_config in config.themes:
                    console.print(f"[blue]Theme:[/blue] {theme_config.name}")
                    suggestion = agent.generate_playlist(theme_config)
                    if suggestion:
                        _print_suggestion(suggestion)
                    else:
                        console.print("[yellow]  Failed to generate suggestion[/yellow]")
                    console.print()
            else:
                results = agent.generate_all_themes()
                for result in results:
                    status = "[green]SUCCESS[/green]" if result["success"] else "[red]FAILED[/red]"
                    console.print(f"Theme: {result['theme']} - {status}")

        elif theme:
            # Find the theme config
            theme_config = next(
                (t for t in config.themes if t.name == theme),
                None,
            )
            if not theme_config:
                console.print(f"[red]Theme '{theme}' not found![/red]")
                console.print("Available themes:")
                for t in config.themes:
                    console.print(f"  - {t.name}")
                raise typer.Exit(1)

            console.print(f"[bold]Generating playlist for theme: {theme}[/bold]\n")
            suggestion = agent.generate_playlist(theme_config)

            if suggestion:
                _print_suggestion(suggestion)
                if not dry_run:
                    if agent.apply_playlist(suggestion):
                        console.print("\n[green]Successfully applied to Tunarr![/green]")
                    else:
                        console.print("\n[red]Failed to apply to Tunarr[/red]")
            else:
                console.print("[red]Failed to generate playlist suggestion[/red]")

        else:
            console.print("[yellow]Specify --theme or --all-themes[/yellow]")
            raise typer.Exit(1)

    finally:
        agent.close()


@app.command()
def scan(
    config_file: Path = typer.Option(
        Path("/app/config/config.yaml"),
        "--config",
        "-c",
        help="Path to configuration file",
    ),
) -> None:
    """Scan media library via Radarr/Sonarr and show available content."""
    config = load_config(config_file)

    if not config.radarr.api_key or not config.sonarr.api_key:
        console.print("[red]Radarr and Sonarr API keys are required![/red]")
        raise typer.Exit(1)

    from .scanner import MediaLibrary

    with MediaLibrary(
        radarr_url=config.radarr.url,
        radarr_api_key=config.radarr.api_key,
        sonarr_url=config.sonarr.url,
        sonarr_api_key=config.sonarr.api_key,
    ) as library:
        # Movies table
        movies_table = Table(title="Top Rated Movies")
        movies_table.add_column("Title", style="cyan")
        movies_table.add_column("Year", style="green")
        movies_table.add_column("Genres", style="yellow")
        movies_table.add_column("IMDB", style="magenta")
        movies_table.add_column("Runtime", style="dim")

        sorted_movies = sorted(library.movies, key=lambda m: m.imdb_rating or 0, reverse=True)
        for movie in sorted_movies[:15]:
            movies_table.add_row(
                movie.title,
                str(movie.year),
                ", ".join(movie.genres[:2]) if movie.genres else "-",
                f"{movie.imdb_rating:.1f}" if movie.imdb_rating else "-",
                f"{movie.runtime}m",
            )

        console.print(movies_table)
        console.print(f"\n[bold]Total movies:[/bold] {len(library.movies)}")

        # Shows table
        shows_table = Table(title="Top Rated TV Shows")
        shows_table.add_column("Title", style="cyan")
        shows_table.add_column("Year", style="green")
        shows_table.add_column("Genres", style="yellow")
        shows_table.add_column("Rating", style="magenta")
        shows_table.add_column("Episodes", style="dim")

        sorted_shows = sorted(library.tv_shows, key=lambda s: s.rating or 0, reverse=True)
        for show in sorted_shows[:10]:
            shows_table.add_row(
                show.title,
                str(show.year),
                ", ".join(show.genres[:2]) if show.genres else "-",
                f"{show.rating:.1f}" if show.rating else "-",
                str(show.episode_count),
            )

        console.print(shows_table)
        console.print(f"[bold]Total TV shows:[/bold] {len(library.tv_shows)}")

        # Anime table
        anime_table = Table(title="Top Rated Anime")
        anime_table.add_column("Title", style="cyan")
        anime_table.add_column("Year", style="green")
        anime_table.add_column("Genres", style="yellow")
        anime_table.add_column("Rating", style="magenta")
        anime_table.add_column("Episodes", style="dim")

        sorted_anime = sorted(library.anime, key=lambda a: a.rating or 0, reverse=True)
        for anime in sorted_anime[:10]:
            genres = ", ".join([g for g in anime.genres[:2] if g.lower() != "anime"]) or "-"
            anime_table.add_row(
                anime.title,
                str(anime.year),
                genres,
                f"{anime.rating:.1f}" if anime.rating else "-",
                str(anime.episode_count),
            )

        console.print(anime_table)
        console.print(f"[bold]Total anime:[/bold] {len(library.anime)}")

        # Genre stats
        console.print("\n[bold]Genre Distribution:[/bold]")
        genre_stats = library.get_genre_stats()
        for genre, count in list(genre_stats.items())[:10]:
            console.print(f"  {genre}: {count}")


@app.command()
def themes(
    config_file: Path = typer.Option(
        Path("/app/config/config.yaml"),
        "--config",
        "-c",
        help="Path to configuration file",
    ),
) -> None:
    """List configured themes."""
    config = load_config(config_file)

    table = Table(title="Configured Themes")
    table.add_column("Name", style="cyan")
    table.add_column("Description", style="green")
    table.add_column("Duration", style="yellow")
    table.add_column("Keywords", style="dim")

    for theme in config.themes:
        table.add_row(
            theme.name,
            theme.description,
            f"{theme.duration}m",
            ", ".join(theme.keywords[:3]) if theme.keywords else "-",
        )

    console.print(table)


def _print_suggestion(suggestion) -> None:
    """Print a playlist suggestion."""
    console.print(f"[bold blue]Custom Show:[/bold blue] {suggestion.collection_name}")

    if suggestion.selected_movies:
        console.print(f"\n[bold]Movies ({len(suggestion.selected_movies)}):[/bold]")
        for title in suggestion.selected_movies:
            console.print(f"  - {title}")

    if suggestion.selected_shows:
        console.print(f"\n[bold]TV Shows ({len(suggestion.selected_shows)}):[/bold]")
        for title in suggestion.selected_shows:
            console.print(f"  - {title}")

    if suggestion.selected_anime:
        console.print(f"\n[bold]Anime ({len(suggestion.selected_anime)}):[/bold]")
        for title in suggestion.selected_anime:
            console.print(f"  - {title}")

    console.print(f"\n[italic]{suggestion.reasoning}[/italic]")
    console.print(f"[dim]Estimated runtime: {suggestion.estimated_runtime} minutes[/dim]")


if __name__ == "__main__":
    app()
