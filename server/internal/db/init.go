package db

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"embed"

	"github.com/adrg/xdg"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var sqlFiles embed.FS

var (
	db       *sql.DB
	dbErr    error
	dbCreate sync.Once
)

// GetDB opens the database once, creating it if needed.
func GetDB() *sql.DB {
	dbCreate.Do(func() {
		db, dbErr = createDB()
		if dbErr != nil {
			log.Fatalf("error getting db: %v", dbErr)
		}
	})
	return db
}

// opens the sqlite database, creating it if needed.
// Note: xdg.DataHome is ~/Libary/Application Support by default on macOS?
func createDB() (*sql.DB, error) {
	// Open database (creates file if it doesn't exist)
	filePath := filepath.Join(xdg.DataHome, "vogo-server", "vogo-server.sqlite")
	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging db: %w", err)
	}

	// Create tables
	schema, _ := sqlFiles.ReadFile("schema.sql")
	_, err = db.Exec(string(schema))
	if err != nil {
		return nil, fmt.Errorf("error creating tables: %w", err)
	}
	return db, nil
}
