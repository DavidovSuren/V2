package models

import "database/sql"

// User — строка таблицы users. Указатели/sql.Null* — для полей, которые
// у свежего пользователя ещё не заполнены (до анкеты, до подписки и т.д.).
type User struct {
	ID         int64
	TgID       string
	Username   sql.NullString
	Name       string
	Gender     sql.NullString
	AgeGroup   string
	PhotosJSON string
	CreatedAt  string

	Level          int
	XP             int
	CompletedCount int

	StreakCurrent int
	StreakBest    int

	DayIndex       int
	LastActionDate sql.NullString

	SubscriptionTier      string
	SubscriptionExpiresAt sql.NullString

	ReferralCode       sql.NullString
	PremiumAgentCode   sql.NullString
	ReferredByUserID   sql.NullInt64
	ReferredByCodeType sql.NullString
}

func (u *User) HasPremiumAgentCode() bool {
	return u.PremiumAgentCode.Valid && u.PremiumAgentCode.String != ""
}
