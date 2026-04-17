from aiogram.fsm.state import State, StatesGroup


class Registration(StatesGroup):
    age = State()
    gender = State()
    city = State()
    interests = State()


class EditProfile(StatesGroup):
    choose_field = State()
    age = State()
    city = State()
    interests = State()
