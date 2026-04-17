import logging
from typing import Optional

from ..domain.match import Match
from ..domain.user import User
from ..infrastructure.matching_client import MatchingClient
from ..infrastructure.user_profile_client import UserProfileClient

logger = logging.getLogger(__name__)


class MatchingUseCases:
    def __init__(
        self,
        matching_client: MatchingClient,
        user_profile_client: UserProfileClient,
    ) -> None:
        self._matching = matching_client
        self._user_profile = user_profile_client

    async def like_profile(
        self,
        from_user_id: int,
        to_user_id: int,
    ) -> tuple[bool, Optional[Match]]:
        """
        Send a like from one user to another.
        Returns (is_match, match) where match is populated on mutual like.
        """
        result = await self._matching.like(from_user_id, to_user_id)
        is_match: bool = result.get("isMatch", False)
        match: Optional[Match] = None
        if is_match and result.get("match"):
            match = Match.from_dict({"match": result["match"]})
        return is_match, match

    async def pass_profile(self, from_user_id: int, to_user_id: int) -> None:
        """Send a pass (skip) from one user to another."""
        await self._matching.pass_(from_user_id, to_user_id)

    async def get_matches(
        self,
        user_id: int,
        page: int = 1,
        page_size: int = 10,
    ) -> list[tuple[Match, Optional[User]]]:
        """
        Return a list of (Match, other_User) pairs for the given user.
        Fetches other user's info from user-profile-service for display purposes.
        """
        data = await self._matching.get_user_matches(
            user_id=user_id,
            page=page,
            page_size=page_size,
        )
        matches_data = data.get("matches", [])

        result: list[tuple[Match, Optional[User]]] = []
        for m_data in matches_data:
            match = Match.from_dict(m_data)
            other_id = match.user2_id if match.user1_id == user_id else match.user1_id
            user_data = await self._user_profile.get_user(other_id)
            other_user = User.from_dict(user_data) if user_data else None
            result.append((match, other_user))

        return result

    async def undo_like(self, from_user_id: int, to_user_id: int) -> None:
        """Remove a previously sent like (user changed their mind)."""
        await self._matching.undo_like(from_user_id, to_user_id)
