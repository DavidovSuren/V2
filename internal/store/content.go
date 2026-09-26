package store

import (
	"encoding/json"
	"time"

	"version20/internal/models"
)

// SeedQuestions досеивает вопросы из кода, которых ещё нет в БД. Уже
// отредактированные в админке не перезаписываются.
func (s *Store) SeedQuestions(qs []models.Question) error {
	for _, q := range qs {
		opts, err := json.Marshal(q.Options)
		if err != nil {
			return err
		}
		if _, err := s.DB.Exec(`
			INSERT INTO questions (id, category, text, type, diagnostic, options_json)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO NOTHING
		`, q.ID, q.Category, q.Text, q.Type, q.Diagnostic, string(opts)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListQuestions() ([]models.Question, error) {
	rows, err := s.DB.Query(`SELECT id, category, text, type, diagnostic, options_json FROM questions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Question
	for rows.Next() {
		var q models.Question
		var opts string
		if err := rows.Scan(&q.ID, &q.Category, &q.Text, &q.Type, &q.Diagnostic, &opts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(opts), &q.Options); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) UpdateQuestion(id int, text string, options []models.Option) error {
	opts, err := json.Marshal(options)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE questions SET text = $1, options_json = $2, updated_at = $3 WHERE id = $4`,
		text, string(opts), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// SeedTasks досеивает задания банка (male/female), которых ещё нет в БД.
// position сохраняет порядок из JSON — от него зависит BuildSchedule.
func (s *Store) SeedTasks(bank string, tasks []models.Task) error {
	for i, t := range tasks {
		if _, err := s.DB.Exec(`
			INSERT INTO tasks (bank, id, position, category, gender, text, why)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (bank, id) DO NOTHING
		`, bank, t.ID, i, t.Category, t.Gender, t.Text, t.Why); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListTasks(bank string) ([]models.Task, error) {
	rows, err := s.DB.Query(`SELECT id, category, gender, text, why FROM tasks WHERE bank = $1 ORDER BY position, id`, bank)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.Category, &t.Gender, &t.Text, &t.Why); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTask(bank, id, text, why string) (bool, error) {
	res, err := s.DB.Exec(`UPDATE tasks SET text = $1, why = $2, updated_at = $3 WHERE bank = $4 AND id = $5`,
		text, why, time.Now().UTC().Format(time.RFC3339), bank, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
