from dataclasses import dataclass


@dataclass
class MediaItem:
    id: int
    user_id: int
    url: str
    original_filename: str
    mime_type: str
    file_size: int
    is_main: bool
    uploaded_at: str

    @classmethod
    def from_dict(cls, data: dict) -> "MediaItem":
        return cls(
            id=int(data.get("id", 0)),
            user_id=int(data.get("user_id", 0)),
            url=data.get("url", ""),
            original_filename=data.get("original_filename", ""),
            mime_type=data.get("mime_type", ""),
            file_size=int(data.get("file_size", 0)),
            is_main=bool(data.get("is_main", False)),
            uploaded_at=data.get("uploaded_at", ""),
        )
