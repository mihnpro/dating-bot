import logging
from typing import Optional

from .base_client import BaseClient

logger = logging.getLogger(__name__)


class MatchingClient(BaseClient):
    """HTTP client for matching-service (via grpc-gateway REST API)."""

    async def like(self, from_user_id: int, to_user_id: int) -> dict:
        """Send a like from one user to another. Returns {isMatch, match?, interaction}."""
        return await self._post(
            "/api/v1/matching/like",
            json={"fromUserId": from_user_id, "toUserId": to_user_id},
        )

    async def pass_(self, from_user_id: int, to_user_id: int) -> dict:
        """Send a pass (skip/dislike) from one user to another. Returns {interaction}."""
        return await self._post(
            "/api/v1/matching/pass",
            json={"fromUserId": from_user_id, "toUserId": to_user_id},
        )

    async def undo_like(self, from_user_id: int, to_user_id: int) -> None:
        """Remove a previously sent like."""
        await self._delete(f"/api/v1/matching/like/{from_user_id}/{to_user_id}")

    async def get_match(self, match_id: int) -> Optional[dict]:
        """Retrieve a specific match by ID."""
        try:
            return await self._get(f"/api/v1/matches/{match_id}")
        except Exception as e:
            logger.warning("get_match(%s) failed: %s", match_id, e)
            return None

    async def get_user_matches(
        self,
        user_id: int,
        page: int = 1,
        page_size: int = 10,
        status: Optional[str] = None,
    ) -> dict:
        """Return all matches for a user. Returns {matches, totalCount, page, pageSize}."""
        params: dict = {"page": page, "pageSize": page_size}
        if status:
            params["status"] = status
        return await self._get(f"/api/v1/matches/user/{user_id}", params=params)

    async def has_matched(self, user1_id: int, user2_id: int) -> dict:
        """Check if two users have a mutual match. Returns {matched, matchId?}."""
        return await self._get(f"/api/v1/matching/check/{user1_id}/{user2_id}")

    async def get_interaction_history(
        self,
        user_id: int,
        page: int = 1,
        page_size: int = 100,
        type_: Optional[str] = None,
    ) -> dict:
        """Return like/pass history for a user. Returns {interactions, totalCount, page, pageSize}."""
        params: dict = {"page": page, "pageSize": page_size}
        if type_:
            params["type"] = type_
        return await self._get(f"/api/v1/matching/history/{user_id}", params=params)

    async def get_who_liked_me(self, user_id: int, page: int = 1, page_size: int = 50) -> dict:
        """Return IDs of users who liked me but I haven't responded to yet."""
        return await self._get(
            f"/api/v1/matching/who-liked-me/{user_id}",
            params={"page": page, "pageSize": page_size},
        )

    async def mark_conversation_started(self, match_id: int) -> None:
        """Mark a match as having started a conversation."""
        await self._post(f"/api/v1/matches/{match_id}/conversation-started")
