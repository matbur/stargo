// Package db wraps the SQLite connection used by the example.
//
// Features own their tables: each data package exports a schema script that
// startup applies once, plus plain query functions that take a *DB. This
// package never grows when a feature is added.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB is the shared SQLite handle. It is safe for concurrent use.
type DB struct {
	*sql.DB
}

// Open opens the database at path and verifies the connection.
//
// An in-memory database is limited to one connection: SQLite gives every
// connection its own private in-memory database, so a pool would see
// different tables on different connections.
func Open(path string) (*DB, error) {
	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if path == ":memory:" {
		handle.SetMaxOpenConns(1)
		handle.SetMaxIdleConns(1)
	}

	if err := handle.Ping(); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return &DB{DB: handle}, nil
}

// ApplySchema runs a feature schema script. Scripts must use idempotent DDL.
func (d *DB) ApplySchema(script string) error {
	if _, err := d.Exec(script); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
