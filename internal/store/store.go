// Package store — весь доступ к БД (репозиторий), по одному файлу на
// область, зеркалящую бывшие backend/routes/*.js. Каждый Store-метод —
// прямой порт соответствующего SQL-запроса из Node-версии.
package store

import "database/sql"

type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{DB: db}
}

// Queryer — общий интерфейс для *sql.DB и *sql.Tx, чтобы методы можно было
// вызывать как внутри транзакции, так и напрямую.
type Queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}
