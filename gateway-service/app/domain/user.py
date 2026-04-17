from dataclasses import dataclass
from typing import Optional


@dataclass
class User:
    id: int
    telegram_id: int
    username: str
    first_name: str
    last_name: str
    status: str
    referral_by: Optional[int] = None

    @classmethod
    def from_dict(cls, data: dict) -> "User":
        u = data.get("user", data)
        return cls(
            id=int(u["id"]),
            telegram_id=int(u["telegramId"]),
            username=u.get("username", ""),
            first_name=u.get("firstName", ""),
            last_name=u.get("lastName", ""),
            status=u.get("status", "active"),
            referral_by=int(u["referralBy"]) if u.get("referralBy") else None,
        )
