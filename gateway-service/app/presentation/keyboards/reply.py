from aiogram.types import KeyboardButton, ReplyKeyboardMarkup, ReplyKeyboardRemove

remove_keyboard = ReplyKeyboardRemove()


def main_menu_keyboard() -> ReplyKeyboardMarkup:
    return ReplyKeyboardMarkup(
        keyboard=[
            [
                KeyboardButton(text="👤 My Profile"),
                KeyboardButton(text="🔍 Browse"),
            ],
            [
                KeyboardButton(text="💞 My Matches"),
            ],
        ],
        resize_keyboard=True,
        input_field_placeholder="Choose action...",
    )


def gender_keyboard() -> ReplyKeyboardMarkup:
    return ReplyKeyboardMarkup(
        keyboard=[
            [
                KeyboardButton(text="👨 Male"),
                KeyboardButton(text="👩 Female"),
            ],
        ],
        resize_keyboard=True,
        one_time_keyboard=True,
        input_field_placeholder="Select your gender...",
    )


def edit_field_keyboard() -> ReplyKeyboardMarkup:
    return ReplyKeyboardMarkup(
        keyboard=[
            [KeyboardButton(text="🎂 Age")],
            [KeyboardButton(text="🏙 City")],
            [KeyboardButton(text="🎯 Interests")],
            [KeyboardButton(text="❌ Cancel")],
        ],
        resize_keyboard=True,
        one_time_keyboard=True,
        input_field_placeholder="What to edit?",
    )
