package main

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'pending',
			html TEXT NOT NULL DEFAULT '',
			preview BLOB,
			one_time_password TEXT,
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	return err
}

func (db *DB) CreateTask(task *Task) error {
	_, err := db.conn.Exec(
		`INSERT INTO tasks (id, status, preview, one_time_password, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		task.ID, task.Status, task.Preview, task.TaskPassword,
		task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (db *DB) GetTask(id string) (*Task, error) {
	row := db.conn.QueryRow(
		`SELECT id, status, html, preview, one_time_password, error, created_at, updated_at
		 FROM tasks WHERE id = ?`, id,
	)
	var t Task
	var pw sql.NullString
	err := row.Scan(
		&t.ID, &t.Status, &t.HTML, &t.Preview,
		&pw, &t.Error,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if pw.Valid {
		t.TaskPassword = pw.String
	}
	return &t, nil
}

func (db *DB) UpdateTaskResult(id string, status TaskStatus, html string, errMsg string) error {
	_, err := db.conn.Exec(
		`UPDATE tasks SET status = ?, html = ?, error = ?, updated_at = ? WHERE id = ?`,
		status, html, errMsg, time.Now(), id,
	)
	return err
}

func (db *DB) Close() error {
	return db.conn.Close()
}
