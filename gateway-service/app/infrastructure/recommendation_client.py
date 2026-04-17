import logging
from typing import Optional

from .base_client import BaseClient

logger = logging.getLogger(__name__)


class RecommendationClient(BaseClient):
    """HTTP client for recommendation-service via grpc-gateway REST endpoints."""

    async def get_next_profile(self, user_id: int) -> Optional[dict]:
        """
        GET /api/v1/recommendations/{user_id}/next

        Returns the enriched profile dict when has_profile is true,
        or None when the recommendation queue is empty.
        """
        try:
            data = await self._get(f"/api/v1/recommendations/{user_id}/next")
            if not data.get("hasProfile", False):
                return None
            return data.get("profile")
        except Exception as exc:
            logger.warning("get_next_profile user_id=%s failed: %s", user_id, exc)
            return None

    async def get_recommendations(self, user_id: int, limit: int = 10) -> list[dict]:
        """
        GET /api/v1/recommendations/{user_id}?limit={limit}

        Returns a ranked list of profile dicts (may be empty).
        """
        try:
            data = await self._get(
                f"/api/v1/recommendations/{user_id}",
                params={"limit": limit},
            )
            return data.get("profiles", [])
        except Exception as exc:
            logger.warning(
                "get_recommendations user_id=%s limit=%s failed: %s",
                user_id,
                limit,
                exc,
            )
            return []

    async def get_rating(self, user_id: int) -> Optional[dict]:
        """
        GET /api/v1/ratings/{user_id}

        Returns the rating dict or None when not found.
        """
        try:
            data = await self._get(f"/api/v1/ratings/{user_id}")
            return data.get("rating")
        except Exception as exc:
            logger.warning("get_rating user_id=%s failed: %s", user_id, exc)
            return None

    async def trigger_recalculation(self, user_id: int) -> bool:
        """
        POST /api/v1/ratings/{user_id}/recalculate

        Enqueues an async full recalculation for the user.
        Returns True on success, False on error.
        """
        try:
            await self._post(f"/api/v1/ratings/{user_id}/recalculate")
            return True
        except Exception as exc:
            logger.warning("trigger_recalculation user_id=%s failed: %s", user_id, exc)
            return False
