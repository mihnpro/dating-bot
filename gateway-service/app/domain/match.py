from dataclasses import dataclass


@dataclass
class Match:
    id: int
    user1_id: int
    user2_id: int
    status: str
    conversation_started: bool

    @classmethod
    def from_dict(cls, data: dict) -> "Match":
        m = data.get("match", data)
        return cls(
            id=int(m["id"]),
            user1_id=int(m["user1Id"]),
            user2_id=int(m["user2Id"]),
            status=m.get("status", "active"),
            conversation_started=m.get("conversationStarted", False),
        )
