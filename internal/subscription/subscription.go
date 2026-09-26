// Package subscription — проверка активности тарифа и пробного периода.
package subscription

import (
	"database/sql"
	"time"
)

// TrialDays — сколько дней после регистрации приложение доступно бесплатно
// любому пользователю (пришёл по ссылке или сам).
const TrialDays = 3

func isTierActive(tier, wantTier string, expiresAt sql.NullString) bool {
	if tier != wantTier || !expiresAt.Valid {
		return false
	}
	t, err := time.Parse(time.RFC3339, expiresAt.String)
	if err != nil {
		return false
	}
	return t.After(time.Now())
}

func IsPremiumActive(tier string, expiresAt sql.NullString) bool {
	return isTierActive(tier, "premium888", expiresAt)
}

func IsAnySubscriptionActive(tier string, expiresAt sql.NullString) bool {
	return isTierActive(tier, "plus369", expiresAt) || isTierActive(tier, "premium888", expiresAt)
}

// ActiveTier — "plus369" / "premium888" или "" если подписки нет.
func ActiveTier(tier string, expiresAt sql.NullString) string {
	if IsAnySubscriptionActive(tier, expiresAt) {
		return tier
	}
	return ""
}

// TrialEnds — момент окончания пробного периода.
func TrialEnds(createdAt string) time.Time {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return time.Time{}
	}
	return t.Add(TrialDays * 24 * time.Hour)
}

// TrialDaysLeft — сколько (неполных) дней пробного периода осталось; 0 — закончился.
func TrialDaysLeft(createdAt string, now time.Time) int {
	left := TrialEnds(createdAt).Sub(now)
	if left <= 0 {
		return 0
	}
	return int((left + 24*time.Hour - time.Nanosecond) / (24 * time.Hour))
}

// HasAccess — пробный период ещё идёт или есть активная подписка.
func HasAccess(tier string, expiresAt sql.NullString, createdAt string, now time.Time) bool {
	return IsAnySubscriptionActive(tier, expiresAt) || TrialEnds(createdAt).After(now)
}
