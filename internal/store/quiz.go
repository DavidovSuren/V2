package store

import (
	"database/sql"
	"encoding/json"
)

// GetQuizAnswer возвращает сохранённый ответ как обычную строку (значения
// хранятся как JSON-строка — см. handlers.mustJSON). Пустая строка, если
// ответа ещё нет.
func (s *Store) GetQuizAnswer(userID int64, questionID int) (string, error) {
	var raw string
	err := s.DB.QueryRow(
		"SELECT answer_json FROM quiz_answers WHERE user_id = $1 AND question_id = $2",
		userID, questionID,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var val string
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return raw, nil // на всякий случай, если когда-то сохранили не-строку
	}
	return val, nil
}
