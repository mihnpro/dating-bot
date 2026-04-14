package entity

import "time"

type User struct {
	ID           int64
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	RegisteredAt time.Time
	LastActive   time.Time
	ReferralBy   *int64
	Status       UserStatus
}

type UserStatus string

const (
	UserStatusActive      UserStatus = "active"
	UserStatusDeactivated UserStatus = "deactivated"
	UserStatusBanned      UserStatus = "banned"
)

func NewUser(telegramID int64, username, firstName, lastName string, referralBy *int64) *User {
	now := time.Now()
	return &User{
		TelegramID:   telegramID,
		Username:     username,
		FirstName:    firstName,
		LastName:     lastName,
		RegisteredAt: now,
		LastActive:   now,
		ReferralBy:   referralBy,
		Status:       UserStatusActive,
	}
}

func (u *User) Deactivate() {
	u.Status = UserStatusDeactivated
}

func (u *User) Ban() {
	u.Status = UserStatusBanned
}

func (u *User) UpdateActivity() {
	u.LastActive = time.Now()
}
