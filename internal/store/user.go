package store

import (
	"database/sql"
	"errors"

	"version20/internal/models"
)

var ErrNotFound = sql.ErrNoRows

const userColumns = `id, tg_id, username, name, gender, age_group, photos_json, created_at,
	level, xp, completed_count, streak_current, streak_best, day_index, last_action_date,
	subscription_tier, subscription_expires_at, referral_code, premium_agent_code,
	referred_by_user_id, referred_by_code_type`

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.TgID, &u.Username, &u.Name, &u.Gender, &u.AgeGroup, &u.PhotosJSON, &u.CreatedAt,
		&u.Level, &u.XP, &u.CompletedCount, &u.StreakCurrent, &u.StreakBest, &u.DayIndex, &u.LastActionDate,
		&u.SubscriptionTier, &u.SubscriptionExpiresAt, &u.ReferralCode, &u.PremiumAgentCode,
		&u.ReferredByUserID, &u.ReferredByCodeType)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByTgID(tgID string) (*models.User, error) {
	row := s.DB.QueryRow("SELECT "+userColumns+" FROM users WHERE tg_id = $1", tgID)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByID(id int64) (*models.User, error) {
	row := s.DB.QueryRow("SELECT "+userColumns+" FROM users WHERE id = $1", id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByUsername(username string) (*models.User, error) {
	row := s.DB.QueryRow("SELECT "+userColumns+" FROM users WHERE username = $1", username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByReferralOrAgentCode(code string) (*models.User, error) {
	row := s.DB.QueryRow("SELECT "+userColumns+" FROM users WHERE referral_code = $1 OR premium_agent_code = $1", code)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (s *Store) ReferralCodeExists(code string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE referral_code = $1)", code).Scan(&exists)
	return exists, err
}

func (s *Store) PremiumAgentCodeExists(code string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE premium_agent_code = $1)", code).Scan(&exists)
	return exists, err
}

func (s *Store) UpdateUsername(userID int64, username string) error {
	_, err := s.DB.Exec("UPDATE users SET username = $1 WHERE id = $2", username, userID)
	return err
}

func (s *Store) UpdateProfile(userID int64, name, ageGroup, photosJSON string) error {
	_, err := s.DB.Exec("UPDATE users SET name = $1, age_group = $2, photos_json = $3 WHERE id = $4",
		name, ageGroup, photosJSON, userID)
	return err
}

type NewUser struct {
	TgID               string
	Username           sql.NullString
	Name               string
	AgeGroup           string
	PhotosJSON         string
	CreatedAt          string
	ReferralCode       string
	ReferredByUserID   sql.NullInt64
	ReferredByCodeType sql.NullString
}

func (s *Store) CreateUser(u NewUser) (int64, error) {
	var id int64
	err := s.DB.QueryRow(`
		INSERT INTO users (tg_id, username, name, age_group, photos_json, created_at,
		                    referral_code, referred_by_user_id, referred_by_code_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, u.TgID, u.Username, u.Name, u.AgeGroup, u.PhotosJSON, u.CreatedAt,
		u.ReferralCode, u.ReferredByUserID, u.ReferredByCodeType).Scan(&id)
	return id, err
}

func (s *Store) SetPremiumAgentCode(userID int64, code string) error {
	_, err := s.DB.Exec("UPDATE users SET premium_agent_code = $1 WHERE id = $2", code, userID)
	return err
}
