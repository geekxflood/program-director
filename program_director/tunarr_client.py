"""Tunarr API client for custom show management."""

from dataclasses import dataclass, field
from typing import Any

import httpx


@dataclass
class CustomShowProgram:
    """Represents a program in a Tunarr custom show."""

    type: str  # 'content' or 'custom'
    subtype: str  # 'movie', 'episode', 'track'
    title: str
    duration: int  # in milliseconds
    external_source_type: str = "plex"  # 'plex', 'jellyfin', 'emby'
    external_source_name: str = ""
    external_source_id: str = ""
    external_key: str = ""
    unique_id: str = ""
    external_ids: list[dict[str, str]] = field(default_factory=list)
    persisted: bool = False
    id: str = ""


@dataclass
class CustomShow:
    """Represents a Tunarr custom show."""

    id: str
    name: str
    content_count: int = 0
    total_duration: int = 0  # in milliseconds


class TunarrClient:
    """Client for Tunarr REST API."""

    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")
        self.client = httpx.Client(timeout=30.0)

    def _get(self, endpoint: str) -> list | dict:
        """Make a GET request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.get(url)
        response.raise_for_status()
        return response.json()

    def _post(self, endpoint: str, data: dict) -> dict:
        """Make a POST request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.post(url, json=data)
        if response.status_code >= 400:
            print(f"POST {endpoint} failed: {response.status_code}")
            print(f"Response: {response.text}")
        response.raise_for_status()
        return response.json()

    def _put(self, endpoint: str, data: dict) -> dict:
        """Make a PUT request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.put(url, json=data)
        if response.status_code >= 400:
            print(f"PUT {endpoint} failed: {response.status_code}")
            print(f"Response: {response.text}")
        response.raise_for_status()
        return response.json()

    def _delete(self, endpoint: str) -> None:
        """Make a DELETE request to the API."""
        url = f"{self.base_url}{endpoint}"
        response = self.client.delete(url)
        response.raise_for_status()

    def get_custom_shows(self) -> list[CustomShow]:
        """Get all custom shows."""
        data = self._get("/api/v2/custom-shows")
        return [
            CustomShow(
                id=item.get("id", ""),
                name=item.get("name", ""),
                content_count=item.get("contentCount", 0),
                total_duration=item.get("totalDuration", 0),
            )
            for item in data
        ]

    def get_custom_show(self, show_id: str) -> CustomShow | None:
        """Get a specific custom show by ID."""
        try:
            data = self._get(f"/api/v2/custom-shows/{show_id}")
            return CustomShow(
                id=data.get("id", ""),
                name=data.get("name", ""),
                content_count=data.get("contentCount", 0),
                total_duration=data.get("totalDuration", 0),
            )
        except httpx.HTTPStatusError as e:
            if e.response.status_code == 404:
                return None
            raise

    def create_custom_show(
        self, name: str, programs: list[dict[str, Any]]
    ) -> CustomShow | None:
        """Create a new custom show with the given programs."""
        try:
            data = self._post(
                "/api/v2/custom-shows",
                {"name": name, "programs": programs},
            )
            return CustomShow(
                id=data.get("id", ""),
                name=name,
                content_count=len(programs),
                total_duration=0,
            )
        except Exception as e:
            print(f"Error creating custom show: {e}")
            return None

    def update_custom_show(
        self, show_id: str, name: str | None = None, programs: list[dict[str, Any]] | None = None
    ) -> CustomShow | None:
        """Update an existing custom show."""
        try:
            update_data: dict[str, Any] = {}
            if name is not None:
                update_data["name"] = name
            if programs is not None:
                update_data["programs"] = programs

            data = self._put(f"/api/v2/custom-shows/{show_id}", update_data)
            return CustomShow(
                id=data.get("id", show_id),
                name=data.get("name", name or ""),
                content_count=data.get("contentCount", 0),
                total_duration=data.get("totalDuration", 0),
            )
        except Exception as e:
            print(f"Error updating custom show: {e}")
            return None

    def delete_custom_show(self, show_id: str) -> bool:
        """Delete a custom show."""
        try:
            self._delete(f"/api/v2/custom-shows/{show_id}")
            return True
        except Exception as e:
            print(f"Error deleting custom show: {e}")
            return False

    def close(self) -> None:
        """Close the HTTP client."""
        self.client.close()

    def __enter__(self) -> "TunarrClient":
        return self

    def __exit__(self, *args) -> None:
        self.close()

