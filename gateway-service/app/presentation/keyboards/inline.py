from aiogram.types import InlineKeyboardButton, InlineKeyboardMarkup
from aiogram.utils.keyboard import InlineKeyboardBuilder


def profile_action_keyboard(to_user_id: int) -> InlineKeyboardMarkup:
    """Like / Pass buttons shown under a browsed profile card."""
    builder = InlineKeyboardBuilder()
    builder.row(
        InlineKeyboardButton(text="❤️ Like", callback_data=f"like:{to_user_id}"),
        InlineKeyboardButton(text="👎 Pass", callback_data=f"pass:{to_user_id}"),
    )
    return builder.as_markup()


def photos_keyboard(photos: list) -> InlineKeyboardMarkup:
    """Inline keyboard for managing a user's photos. Each row: view + delete."""
    builder = InlineKeyboardBuilder()
    for idx, photo in enumerate(photos, start=1):
        builder.row(
            InlineKeyboardButton(text=f"📷 Photo {idx}", callback_data=f"photo_view:{photo.id}"),
            InlineKeyboardButton(text="🗑 Delete", callback_data=f"photo_del:{photo.id}"),
        )
    builder.row(
        InlineKeyboardButton(text="➕ Add photo", callback_data="photo_add")
    )
    return builder.as_markup()


def matches_keyboard(matches: list[tuple[int, str]]) -> InlineKeyboardMarkup:
    """
    Build an inline keyboard listing all matches.

    Args:
        matches: list of (match_id, other_user_display_name)
    """
    builder = InlineKeyboardBuilder()
    for match_id, name in matches:
        builder.row(
            InlineKeyboardButton(
                text=f"💬 {name}",
                callback_data=f"match_info:{match_id}",
            )
        )
    return builder.as_markup()
