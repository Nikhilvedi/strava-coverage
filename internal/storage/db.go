package storage

import (
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// DB wraps sqlx.DB to provide custom functionality
type DB struct {
	*sqlx.DB
}

// NewDB creates a new database connection
func NewDB(dbURL string) (*DB, error) {
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	// Set reasonable pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Try to enable PostGIS (no-op if already enabled)
	if _, err := db.Exec("CREATE EXTENSION IF NOT EXISTS postgis"); err != nil {
		return nil, fmt.Errorf("failed to enable PostGIS: %v", err)
	}

	return &DB{db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// QueryRowx wraps sqlx.DB.QueryRowx
func (db *DB) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	return db.DB.QueryRowx(query, args...)
}

// Exec wraps sqlx.DB.Exec
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(query, args...)
}

// Select wraps sqlx.DB.Select
func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	return db.DB.Select(dest, query, args...)
}
