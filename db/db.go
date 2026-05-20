package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
)

// Connect открывает соединение с PostgreSQL и создаёт таблицу если не существует.
func Connect(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("db ping failed: %w", err)
	}

	if err := migrate(conn); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return conn, nil
}

// migrate создаёт таблицу services если не существует.
func migrate(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS services (
		name        VARCHAR(100) PRIMARY KEY,
		description TEXT         NOT NULL DEFAULT '',
		script      TEXT         NOT NULL,
		created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
	);`
	_, err := db.Exec(query)
	return err
}

