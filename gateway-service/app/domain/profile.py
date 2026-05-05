from dataclasses import dataclass, field
from typing import List


@dataclass
class Profile:
    id: int
    user_id: int
    age: int
    gender: str
    city: str
    interests: List[str] = field(default_factory=list)
    photos_count: int = 0
    fullness_percent: float = 0.0

    @classmethod
    def from_dict(cls, data: dict) -> "Profile":
        p = data.get("profile", data)
        return cls(
            id=int(p.get("id", 0)),
            user_id=int(p["userId"]),
            age=int(p.get("age", 0)),
            gender=p.get("gender", ""),
            city=p.get("city", ""),
            interests=p.get("interests", []),
            photos_count=int(p.get("photosCount", 0)),
            fullness_percent=float(p.get("fullnessPercent", 0)),
        )

    def format_card(self, name: str = "") -> str:
        interests_str = ", ".join(self.interests) if self.interests else "—"
        gender_emoji = "👨" if self.gender.lower() == "male" else "👩"
        photos_str = f"📷 {self.photos_count} photo{'s' if self.photos_count != 1 else ''}" if self.photos_count else "📷 No photos yet"
        fullness = int(self.fullness_percent * 100)
        return (
            f"{gender_emoji} <b>{name or 'User'}</b>\n"
            f"🎂 Age: <b>{self.age}</b>\n"
            f"🏙 City: <b>{self.city}</b>\n"
            f"🎯 Interests: {interests_str}\n"
            f"{photos_str}  •  Profile: <b>{fullness}%</b> complete"
        )
