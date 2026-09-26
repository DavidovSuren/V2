package store

import (
	"database/sql"
	"errors"
)

// AdminUserRow — строка списка пользователей в админке.
type AdminUserRow struct {
	ID                    int64
	TgID                  string
	Username              sql.NullString
	Name                  string
	Level                 int
	StreakCurrent         int
	SubscriptionTier      string
	SubscriptionExpiresAt sql.NullString
	PremiumAgentCode      sql.NullString
	CreatedAt             string
}

// SearchUsers ищет по имени, @username, tg_id или коду (пустой запрос — все).
func (s *Store) SearchUsers(q string, limit int) ([]AdminUserRow, error) {
	rows, err := s.DB.Query(`
		SELECT id, tg_id, username, name, level, streak_current, subscription_tier,
		       subscription_expires_at, premium_agent_code, created_at
		FROM users
		WHERE $1 = '' OR name ILIKE '%' || $1 || '%' OR username ILIKE '%' || $1 || '%'
		   OR tg_id = $1 OR referral_code = $1 OR premium_agent_code = $1
		ORDER BY id DESC
		LIMIT $2
	`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AdminUserRow
	for rows.Next() {
		var u AdminUserRow
		if err := rows.Scan(&u.ID, &u.TgID, &u.Username, &u.Name, &u.Level, &u.StreakCurrent, &u.SubscriptionTier,
			&u.SubscriptionExpiresAt, &u.PremiumAgentCode, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// AdminUpdateUser — правка пользователя из админки. expiresAt пустой —
// подписка без даты (для free).
func (s *Store) AdminUpdateUser(id int64, name, username, tier, expiresAt string) error {
	_, err := s.DB.Exec(`
		UPDATE users SET name = $1, username = NULLIF($2, ''), subscription_tier = $3,
		       subscription_expires_at = NULLIF($4, '')
		WHERE id = $5
	`, name, username, tier, expiresAt, id)
	return err
}

type AdminStats struct {
	Users, Onboarded, Plus, Premium, Partners int
	Revenue, Commissions                      int
}

func (s *Store) Stats(nowRFC3339 string) (AdminStats, error) {
	var st AdminStats
	err := s.DB.QueryRow(`
		SELECT
		  (SELECT COUNT(*) FROM users)::int,
		  (SELECT COUNT(*) FROM users WHERE gender IS NOT NULL)::int,
		  (SELECT COUNT(*) FROM users WHERE subscription_tier = 'plus369' AND subscription_expires_at > $1)::int,
		  (SELECT COUNT(*) FROM users WHERE subscription_tier = 'premium888' AND subscription_expires_at > $1)::int,
		  (SELECT COUNT(DISTINCT referrer_commission_user_id) FROM subscription_payments WHERE commission_amount > 0)::int,
		  (SELECT COALESCE(SUM(price_paid), 0) FROM subscription_payments)::int,
		  (SELECT COALESCE(SUM(commission_amount), 0) FROM subscription_payments)::int
	`, nowRFC3339).Scan(&st.Users, &st.Onboarded, &st.Plus, &st.Premium, &st.Partners, &st.Revenue, &st.Commissions)
	return st, err
}

func (s *Store) AgentPasswordHash(userID int64) (string, error) {
	var h sql.NullString
	err := s.DB.QueryRow(`SELECT agent_password_hash FROM users WHERE id = $1`, userID).Scan(&h)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return h.String, err
}

func (s *Store) SetAgentPasswordHash(userID int64, hash string) error {
	_, err := s.DB.Exec(`UPDATE users SET agent_password_hash = NULLIF($1, '') WHERE id = $2`, hash, userID)
	return err
}

// ReferredRow — приглашённый пользователем человек (для кабинета партнёра).
type ReferredRow struct {
	Name             string
	JoinedAt         string
	SubscriptionTier string
	Paid             int // сумма оплат
	Commission       int // сколько получил партнёр
}

func (s *Store) PartnerReferred(partnerID int64) ([]ReferredRow, error) {
	rows, err := s.DB.Query(`
		SELECT u.name, u.created_at, u.subscription_tier,
		       COALESCE(SUM(p.price_paid), 0)::int,
		       COALESCE(SUM(CASE WHEN p.referrer_commission_user_id = $1 THEN p.commission_amount ELSE 0 END), 0)::int
		FROM users u
		LEFT JOIN subscription_payments p ON p.user_id = u.id
		WHERE u.referred_by_user_id = $1
		GROUP BY u.id, u.name, u.created_at, u.subscription_tier
		ORDER BY u.created_at DESC
	`, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReferredRow
	for rows.Next() {
		var r ReferredRow
		if err := rows.Scan(&r.Name, &r.JoinedAt, &r.SubscriptionTier, &r.Paid, &r.Commission); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
