package store

import (
	"database/sql"
	"strconv"
	"strings"
)

type PersonRow struct {
	ID      int64
	Name    string
	Gender  sql.NullString
	Level   int
	Badges  []string
	Diamond bool
}

func (s *Store) AllUsersRanked() ([]PersonRow, error) {
	rows, err := s.DB.Query(`SELECT id, name, gender, level FROM users ORDER BY level DESC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PersonRow
	for rows.Next() {
		var p PersonRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.Level); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) FriendsRaw(userID int64) ([]PersonRow, error) {
	rows, err := s.DB.Query(`
		SELECT DISTINCT u.id, u.name, u.gender, u.level FROM users u
		WHERE u.id IN (
			SELECT referred_id FROM referrals WHERE referrer_id = $1
			UNION
			SELECT referrer_id FROM referrals WHERE referred_id = $1
			UNION
			SELECT friend_id FROM friendships WHERE user_id = $1
		)
		ORDER BY u.level DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PersonRow
	for rows.Next() {
		var p PersonRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.Level); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AttachBadges заполняет Badges/Diamond у уже полученных строк.
func (s *Store) AttachBadges(people []PersonRow) ([]PersonRow, error) {
	if len(people) == 0 {
		return people, nil
	}
	ids := make([]any, len(people))
	placeholders := make([]string, len(people))
	for i, p := range people {
		ids[i] = p.ID
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}
	query := "SELECT user_id, code FROM achievements WHERE user_id IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := s.DB.Query(query, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byUser := map[int64][]string{}
	for rows.Next() {
		var userID int64
		var code string
		if err := rows.Scan(&userID, &code); err != nil {
			return nil, err
		}
		byUser[userID] = append(byUser[userID], code)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range people {
		people[i].Badges = byUser[people[i].ID]
		people[i].Diamond = people[i].Level >= 100
	}
	return people, nil
}

func (s *Store) CreateReferral(q Queryer, referrerID, referredID int64, codeType, createdAt string) error {
	_, err := q.Exec(`
		INSERT INTO referrals (referrer_id, referred_id, code_type, created_at) VALUES ($1, $2, $3, $4)
	`, referrerID, referredID, codeType, createdAt)
	return err
}

func (s *Store) AddFriendship(userID, friendID int64, createdAt string) error {
	_, err := s.DB.Exec(`
		INSERT INTO friendships (user_id, friend_id, created_at) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, friend_id) DO NOTHING
	`, userID, friendID, createdAt)
	return err
}

func (s *Store) PriorPaidReferralsCount(referrerID int64) (int, error) {
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(DISTINCT r.referred_id)::int FROM referrals r
		JOIN subscription_payments sp ON sp.user_id = r.referred_id
		WHERE r.referrer_id = $1
	`, referrerID).Scan(&count)
	return count, err
}
