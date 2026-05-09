import asyncio
import logging
from typing import Optional

from aiogram import F, Router
from aiogram.filters import Command
from aiogram.fsm.context import FSMContext
from aiogram.types import BufferedInputFile, CallbackQuery, InputMediaPhoto, Message

from ...application.matching_use_cases import MatchingUseCases
from ...application.media_use_cases import MediaUseCases
from ...application.user_use_cases import UserUseCases
from ...infrastructure.chat_client import ChatClient
from ...infrastructure.recommendation_client import RecommendationClient
from ..keyboards.inline import (
    match_announcement_keyboard,
    match_chat_keyboard,
    matches_keyboard,
    profile_action_keyboard,
    who_liked_me_action_keyboard,
)
from ..keyboards.reply import main_menu_keyboard
from ..states.registration import WhoLikedMe

logger = logging.getLogger(__name__)

router = Router()


async def _build_chat_url(
    chat_client: ChatClient,
    frontend_url: str,
    user_id: int,
    match_id: int,
    other_user_id: int | None = None,
) -> str | None:
    """Create or fetch conversation, then return a signed chat URL. Returns None on error."""
    try:
        if other_user_id is not None:
            await chat_client.get_or_create_conversation(match_id, user_id, other_user_id)
        data = await chat_client.get_token(user_id)
        token = data.get("token", "")
        return f"{frontend_url}/?user_id={user_id}&match_id={match_id}&token={token}"
    except Exception as exc:
        logger.warning("Failed to build chat url for user=%s: %s", user_id, exc)
        return None


# =========================================================================== #
# Helpers                                                                       #
# =========================================================================== #


async def _get_current_user(message: Message, user_use_cases: UserUseCases):
    tg = message.from_user
    return await user_use_cases.get_or_register_user(
        telegram_id=tg.id,
        username=tg.username or "",
        first_name=tg.first_name or "",
        last_name=tg.last_name or "",
    )


def _format_card(profile: dict, name: str = "") -> str:
    """Format a RecommendedProfile dict (from recommendation-service) into
    an HTML card string suitable for Telegram messages."""
    gender = profile.get("gender", "")
    gender_emoji = "👨" if gender.lower() == "male" else "👩"
    interests = profile.get("interests") or []
    interests_str = ", ".join(interests) if interests else "—"
    display_name = name or "User"
    age = profile.get("age", "?")
    city = profile.get("city", "—")
    return (
        f"{gender_emoji} <b>{display_name}</b>\n"
        f"🎂 Age: <b>{age}</b>\n"
        f"🏙 City: <b>{city}</b>\n"
        f"🎯 Interests: {interests_str}"
    )


async def _get_all_photo_bytes(
    user_id: int, media_use_cases: MediaUseCases
) -> list[bytes]:
    try:
        photos = await media_use_cases.get_user_photos(user_id)
        results = await asyncio.gather(
            *(media_use_cases.get_photo_bytes(p.url) for p in photos),
            return_exceptions=True,
        )
        return [r for r in results if isinstance(r, bytes) and r]
    except Exception:
        return []


async def _send_card_with_photo(
    message: Message,
    text: str,
    all_photo_bytes: list[bytes],
    reply_markup=None,
) -> None:
    if len(all_photo_bytes) == 1:
        await message.answer_photo(
            photo=BufferedInputFile(all_photo_bytes[0], filename="photo.jpg"),
            caption=text,
            parse_mode="HTML",
            reply_markup=reply_markup,
        )
    elif len(all_photo_bytes) > 1:
        media = [
            InputMediaPhoto(media=BufferedInputFile(data, filename=f"photo_{i}.jpg"))
            for i, data in enumerate(all_photo_bytes)
        ]
        await message.answer_media_group(media=media)
        await message.answer(text, parse_mode="HTML", reply_markup=reply_markup)
    else:
        await message.answer(text, reply_markup=reply_markup)


async def _show_next_profile(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
    recommendation_client: RecommendationClient,
    current_user_id: int,
) -> None:
    """
    Fetch the next recommended profile from recommendation-service and send
    it as a card with Like / Pass inline buttons.

    recommendation-service maintains the ranked Redis queue per viewer, so
    we simply call GetNextProfile on every swipe — no local FSM queue needed.
    """
    profile_data = await recommendation_client.get_next_profile(current_user_id)

    if profile_data is None:
        await message.answer(
            "😔 No more profiles to show right now. Come back later!",
            reply_markup=main_menu_keyboard(),
        )
        return

    candidate_user_id: int = int(profile_data.get("userId", 0))
    if candidate_user_id == 0:
        await message.answer(
            "😔 No more profiles right now. Check back soon!",
            reply_markup=main_menu_keyboard(),
        )
        return

    other_user = await user_use_cases.get_user(candidate_user_id)
    name = other_user.first_name if other_user else "User"

    card_text = _format_card(profile_data, name=name)
    photo_bytes = await _get_all_photo_bytes(candidate_user_id, media_use_cases)

    await _send_card_with_photo(
        message,
        card_text,
        photo_bytes,
        reply_markup=profile_action_keyboard(candidate_user_id),
    )


# =========================================================================== #
# Browse                                                                        #
# =========================================================================== #


@router.message(Command("browse"))
@router.message(F.text == "🔍 Browse")
async def browse_handler(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    media_use_cases: MediaUseCases,
    recommendation_client: RecommendationClient,
) -> None:
    user = await _get_current_user(message, user_use_cases)
    profile = await user_use_cases.get_profile(user.id)

    if profile is None:
        await message.answer(
            "⚠️ You need a profile before you can browse!\n"
            "Use /createprofile to set one up."
        )
        return

    # Trigger a background recalculation so the viewer's own rating is fresh
    # before they start browsing (fire-and-forget, errors are swallowed).
    await recommendation_client.trigger_recalculation(user.id)

    await _show_next_profile(
        message=message,
        state=state,
        user_use_cases=user_use_cases,
        media_use_cases=media_use_cases,
        recommendation_client=recommendation_client,
        current_user_id=user.id,
    )


# =========================================================================== #
# Like callback                                                                 #
# =========================================================================== #


@router.callback_query(F.data.startswith("like:"))
async def like_callback(
    callback: CallbackQuery,
    state: FSMContext,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    media_use_cases: MediaUseCases,
    recommendation_client: RecommendationClient,
    chat_client: ChatClient,
    chat_frontend_url: str,
) -> None:
    await callback.answer()

    to_user_id = int(callback.data.split(":")[1])
    tg = callback.from_user

    user = await user_use_cases.get_or_register_user(
        telegram_id=tg.id,
        username=tg.username or "",
        first_name=tg.first_name or "",
        last_name=tg.last_name or "",
    )

    try:
        is_match, match = await matching_use_cases.like_profile(
            from_user_id=user.id,
            to_user_id=to_user_id,
        )
    except Exception as exc:
        logger.error(
            "like_profile failed user=%s target=%s: %s", user.id, to_user_id, exc
        )
        await callback.message.answer("❌ Something went wrong. Please try again.")
        return

    if is_match and match:
        other_user = await user_use_cases.get_user(to_user_id)
        other_name = other_user.first_name if other_user else "someone"
        other_username = other_user.username if other_user else None

        chat_url = await _build_chat_url(chat_client, chat_frontend_url, user.id, match.id, to_user_id)

        await callback.message.answer(
            f"🎉 <b>It's a Match!</b>\n\n"
            f"You and <b>{other_name}</b> liked each other!\n\n"
            f"Start a conversation right now 👇",
            parse_mode="HTML",
            reply_markup=match_announcement_keyboard(chat_url, other_username),
        )
    else:
        await callback.message.answer("❤️ Liked!")

    await _show_next_profile(
        message=callback.message,
        state=state,
        user_use_cases=user_use_cases,
        media_use_cases=media_use_cases,
        recommendation_client=recommendation_client,
        current_user_id=user.id,
    )


# =========================================================================== #
# Pass callback                                                                 #
# =========================================================================== #


@router.callback_query(F.data.startswith("pass:"))
async def pass_callback(
    callback: CallbackQuery,
    state: FSMContext,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    media_use_cases: MediaUseCases,
    recommendation_client: RecommendationClient,
) -> None:
    await callback.answer()

    to_user_id = int(callback.data.split(":")[1])
    tg = callback.from_user

    user = await user_use_cases.get_or_register_user(
        telegram_id=tg.id,
        username=tg.username or "",
        first_name=tg.first_name or "",
        last_name=tg.last_name or "",
    )

    try:
        await matching_use_cases.pass_profile(
            from_user_id=user.id,
            to_user_id=to_user_id,
        )
    except Exception as exc:
        logger.error(
            "pass_profile failed user=%s target=%s: %s", user.id, to_user_id, exc
        )

    await _show_next_profile(
        message=callback.message,
        state=state,
        user_use_cases=user_use_cases,
        media_use_cases=media_use_cases,
        recommendation_client=recommendation_client,
        current_user_id=user.id,
    )


# =========================================================================== #
# Matches list                                                                  #
# =========================================================================== #


@router.message(Command("matches"))
@router.message(F.text == "💞 My Matches")
async def matches_handler(
    message: Message,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    recommendation_client: RecommendationClient,
) -> None:
    user = await _get_current_user(message, user_use_cases)

    try:
        matches = await matching_use_cases.get_matches(user.id)
    except Exception as exc:
        logger.error("get_matches failed for user=%s: %s", user.id, exc)
        await message.answer(
            "❌ Could not load matches right now. Please try again.",
            reply_markup=main_menu_keyboard(),
        )
        return

    if not matches:
        await message.answer(
            "💔 No matches yet — keep browsing!\n\n"
            "Use /browse or tap <b>🔍 Browse</b> to discover people.",
            reply_markup=main_menu_keyboard(),
        )
        return

    match_list = [(m.id, u.first_name if u else f"User #{m.id}") for m, u in matches]

    await message.answer(
        f"💞 <b>Your Matches</b> ({len(matches)})\n\nTap a name to see details:",
        reply_markup=matches_keyboard(match_list),
    )


# =========================================================================== #
# Match info callback                                                           #
# =========================================================================== #


@router.callback_query(F.data.startswith("match_info:"))
async def match_info_callback(
    callback: CallbackQuery,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    media_use_cases: MediaUseCases,
    recommendation_client: RecommendationClient,
    chat_client: ChatClient,
    chat_frontend_url: str,
) -> None:
    await callback.answer()

    match_id = int(callback.data.split(":")[1])
    tg = callback.from_user

    user = await user_use_cases.get_or_register_user(
        telegram_id=tg.id,
        username=tg.username or "",
        first_name=tg.first_name or "",
        last_name=tg.last_name or "",
    )

    try:
        matches = await matching_use_cases.get_matches(user.id)
    except Exception as exc:
        logger.error("get_matches failed: %s", exc)
        await callback.message.answer("❌ Could not load match details.")
        return

    matched_pair: Optional[tuple] = next(
        ((m, u) for m, u in matches if m.id == match_id), None
    )

    if matched_pair is None:
        await callback.message.answer("⚠️ Match not found.")
        return

    match, other_user = matched_pair
    other_id = match.user2_id if match.user1_id == user.id else match.user1_id
    other_profile = await user_use_cases.get_profile(other_id)
    other_username = other_user.username if other_user else None

    chat_url = await _build_chat_url(chat_client, chat_frontend_url, user.id, match_id, other_id)
    keyboard = match_chat_keyboard(match_id, chat_url, other_username)

    if other_user and other_profile:
        card = other_profile.format_card(name=other_user.first_name)
        photo_bytes = await _get_all_photo_bytes(other_id, media_use_cases)
        await _send_card_with_photo(
            callback.message,
            f"💞 <b>Your Match</b>\n\n{card}",
            photo_bytes,
            reply_markup=keyboard,
        )
    else:
        await callback.message.answer(
            f"💞 Match #{match_id}",
            reply_markup=keyboard,
        )


# =========================================================================== #
# Who Liked Me                                                                  #
# =========================================================================== #

async def _show_next_who_liked_me(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    data = await state.get_data()
    queue: list[int] = data.get("wlm_queue", [])

    if not queue:
        await state.clear()
        await message.answer(
            "✅ You've reviewed everyone who liked you!",
            reply_markup=main_menu_keyboard(),
        )
        return

    candidate_id = queue[0]
    await state.update_data(wlm_queue=queue[1:])

    other_user = await user_use_cases.get_user(candidate_id)
    name = other_user.first_name if other_user else f"User #{candidate_id}"
    other_profile = await user_use_cases.get_profile(candidate_id)

    if other_profile:
        card = other_profile.format_card(name=name)
    else:
        card = f"👤 <b>{name}</b>"

    photo_bytes = await _get_all_photo_bytes(candidate_id, media_use_cases)
    remaining = len(queue) - 1
    header = f"❤️ <b>Liked you</b>  ({remaining} more)\n\n"
    await _send_card_with_photo(
        message,
        header + card,
        photo_bytes,
        reply_markup=who_liked_me_action_keyboard(candidate_id),
    )


@router.message(F.text == "❤️ Who Liked Me")
async def who_liked_me_handler(
    message: Message,
    state: FSMContext,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    user = await _get_current_user(message, user_use_cases)

    try:
        liked_list = await matching_use_cases.get_who_liked_me(user.id)
    except Exception as exc:
        logger.error("get_who_liked_me failed user=%s: %s", user.id, exc)
        await message.answer("❌ Could not load the list. Please try again.")
        return

    if not liked_list:
        await message.answer(
            "😔 Nobody has liked you yet — keep your profile up to date!",
            reply_markup=main_menu_keyboard(),
        )
        return

    queue = [uid for uid, _ in liked_list]
    await state.set_state(WhoLikedMe.browsing)
    await state.update_data(wlm_queue=queue)

    await message.answer(
        f"❤️ <b>{len(queue)} people liked you!</b>\n\nSwipe through their profiles:",
        parse_mode="HTML",
    )
    await _show_next_who_liked_me(message, state, user_use_cases, media_use_cases)


@router.callback_query(WhoLikedMe.browsing, F.data.startswith("wlm_like:"))
async def wlm_like_callback(
    callback: CallbackQuery,
    state: FSMContext,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    media_use_cases: MediaUseCases,
    chat_client: ChatClient,
    chat_frontend_url: str,
) -> None:
    await callback.answer()
    to_user_id = int(callback.data.split(":")[1])
    user = await _get_current_user(callback.message, user_use_cases)

    try:
        is_match, match = await matching_use_cases.like_profile(
            from_user_id=user.id, to_user_id=to_user_id
        )
    except Exception as exc:
        logger.error("wlm_like failed user=%s target=%s: %s", user.id, to_user_id, exc)
        await callback.message.answer("❌ Something went wrong. Please try again.")
        return

    if is_match and match:
        other_user = await user_use_cases.get_user(to_user_id)
        other_name = other_user.first_name if other_user else "someone"
        other_username = other_user.username if other_user else None
        chat_url = await _build_chat_url(chat_client, chat_frontend_url, user.id, match.id, to_user_id)
        await callback.message.answer(
            f"🎉 <b>It's a Match!</b>\n\nYou and <b>{other_name}</b> liked each other!",
            parse_mode="HTML",
            reply_markup=match_announcement_keyboard(chat_url, other_username),
        )
    else:
        await callback.message.answer("❤️ Liked!")

    await _show_next_who_liked_me(callback.message, state, user_use_cases, media_use_cases)


@router.callback_query(WhoLikedMe.browsing, F.data.startswith("wlm_dislike:"))
async def wlm_dislike_callback(
    callback: CallbackQuery,
    state: FSMContext,
    user_use_cases: UserUseCases,
    matching_use_cases: MatchingUseCases,
    media_use_cases: MediaUseCases,
) -> None:
    await callback.answer()
    to_user_id = int(callback.data.split(":")[1])
    user = await _get_current_user(callback.message, user_use_cases)

    try:
        await matching_use_cases.pass_profile(from_user_id=user.id, to_user_id=to_user_id)
    except Exception as exc:
        logger.error("wlm_dislike failed user=%s target=%s: %s", user.id, to_user_id, exc)

    await _show_next_who_liked_me(callback.message, state, user_use_cases, media_use_cases)


@router.callback_query(WhoLikedMe.browsing, F.data == "wlm_cancel")
async def wlm_cancel_callback(
    callback: CallbackQuery,
    state: FSMContext,
) -> None:
    await callback.answer()
    await state.clear()
    await callback.message.answer(
        "↩️ Cancelled. Back to main menu.",
        reply_markup=main_menu_keyboard(),
    )
