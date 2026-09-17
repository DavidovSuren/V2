// Package subscription — проверка активности тарифа. Порт backend/lib/subscriptionUtil.js.
package subscription

import (
	"database/sql"
	"time"
)

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
