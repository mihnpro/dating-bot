from .base_client import BaseClient


class ChatClient(BaseClient):
    """HTTP client for chat-service."""

    async def get_token(self, user_id: int) -> dict:
        """Returns {"token": "...", "user_id": N}."""
        return await self._get(f"/api/v1/chat/token?user_id={user_id}")

    async def get_or_create_conversation(
        self, match_id: int, user1_id: int, user2_id: int
    ) -> dict:
        """Returns conversation object with ID field."""
        return await self._post(
            "/api/v1/chat/conversations",
            {"match_id": match_id, "user1_id": user1_id, "user2_id": user2_id},
        )
