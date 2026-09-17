package store

import (
	"database/sql"
	"errors"

	"version20/internal/models"
)

func (s *Store) HasSchedule(userID int64) (bool, error) {
	var exists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM user_schedule WHERE user_id = $1)", userID).Scan(&exists)
	return exists, err
}

func (s *Store) UpsertQuizAnswer(q Queryer, userID int64, questionID int, answerJSON string) error {
	_, err := q.Exec(`
		INSERT INTO quiz_answers (user_id, question_id, answer_json) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, question_id) DO UPDATE SET answer_json = excluded.answer_json
	`, userID, questionID, answerJSON)
	return err
}

func (s *Store) UpsertCategoryWeight(q Queryer, userID int64, category string, weight float64) error {
	_, err := q.Exec(`
		INSERT INTO category_weights (user_id, category, weight) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, category) DO UPDATE SET weight = excluded.weight
	`, userID, category, weight)
	return err
}

func (s *Store) InsertScheduleRow(q Queryer, userID int64, dayIndex int, t models.Task) error {
	_, err := q.Exec(`
		INSERT INTO user_schedule (user_id, day_index, task_id, category, task_text, task_why, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
	`, userID, dayIndex, t.ID, t.Category, t.Text, t.Why)
	return err
}

func (s *Store) FinishQuiz(q Queryer, userID int64, gender string, xpGain int) error {
	_, err := q.Exec("UPDATE users SET gender = $1, day_index = 0, xp = xp + $2 WHERE id = $3", gender, xpGain, userID)
	return err
}

type ScheduleRow struct {
	TaskID   string
	Category string
	Text     string
	Why      string
	Status   string
}

func (s *Store) GetScheduleRow(userID int64, dayIndex int) (*ScheduleRow, error) {
	row := s.DB.QueryRow(`
		SELECT task_id, category, task_text, task_why, status FROM user_schedule
		WHERE user_id = $1 AND day_index = $2
	`, userID, dayIndex)
	var r ScheduleRow
	err := row.Scan(&r.TaskID, &r.Category, &r.Text, &r.Why, &r.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) MarkSkip(userID int64, today string) error {
	_, err := s.DB.Exec("UPDATE users SET last_action_date = $1, streak_current = 0 WHERE id = $2", today, userID)
	return err
}

func (s *Store) InsertActionLog(q Queryer, userID int64, date, action, category string) error {
	_, err := q.Exec(`
		INSERT INTO action_log (user_id, action_date, action, category) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, action_date) DO NOTHING
	`, userID, date, action, category)
	return err
}

func (s *Store) UpdateScheduleStatus(q Queryer, userID int64, dayIndex int, status string) error {
	_, err := q.Exec("UPDATE user_schedule SET status = $1 WHERE user_id = $2 AND day_index = $3", status, userID, dayIndex)
	return err
}

func (s *Store) InsertDiaryEntry(q Queryer, userID int64, date, emoji, note string, xpAwarded int) error {
	_, err := q.Exec(`
		INSERT INTO diary_entries (user_id, entry_date, emoji, note, xp_awarded) VALUES ($1, $2, $3, $4, $5)
	`, userID, date, emoji, note, xpAwarded)
	return err
}

func (s *Store) ApplyCompletion(q Queryer, userID int64, completedCount, xpGain, level, streakCurrent, streakBest int, today string) error {
	_, err := q.Exec(`
		UPDATE users SET
			completed_count = $1, xp = xp + $2, level = $3,
			streak_current = $4, streak_best = $5,
			day_index = day_index + 1, last_action_date = $6
		WHERE id = $7
	`, completedCount, xpGain, level, streakCurrent, streakBest, today, userID)
	return err
}

func (s *Store) GrantLevel100Premium(q Queryer, userID int64, expiresAt string) error {
	_, err := q.Exec(`
		UPDATE users SET subscription_tier = 'premium888', subscription_expires_at = $1 WHERE id = $2
	`, expiresAt, userID)
	return err
}

type DiaryEntry struct {
	Date  string
	Emoji string
	Note  sql.NullString
}

func (s *Store) DiaryHistory(userID int64, limit int) ([]DiaryEntry, error) {
	rows, err := s.DB.Query(`
		SELECT entry_date, emoji, note FROM diary_entries
		WHERE user_id = $1 ORDER BY entry_date DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DiaryEntry
	for rows.Next() {
		var e DiaryEntry
		if err := rows.Scan(&e.Date, &e.Emoji, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type CategoryDone struct {
	Category string
	Done     int
}

func (s *Store) DoneCountsByCategory(userID int64) ([]CategoryDone, error) {
	rows, err := s.DB.Query(`
		SELECT category, COUNT(*)::int as done FROM user_schedule
		WHERE user_id = $1 AND status = 'done' GROUP BY category
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CategoryDone
	for rows.Next() {
		var c CategoryDone
		if err := rows.Scan(&c.Category, &c.Done); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
