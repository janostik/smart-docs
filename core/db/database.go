package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Service interface {
	Health() map[string]string
	Close() error
}

type service struct {
	db *sql.DB
}

var (
	dbInstance *service
)

func New() Service {
	if dbInstance != nil {
		return dbInstance
	}
	db, err := sql.Open("sqlite3", "./data/documents.sql")
	if err != nil {
		// This will not be connection error
		log.Fatal(err)
	}
	dbInstance = &service{
		db: db,
	}

	// Run migrations
	if err := dbInstance.runMigrations(); err != nil {
		log.Fatal(err)
	}

	return dbInstance
}

func (s *service) runMigrations() error {
	// Create migrations table if it doesn't exist
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %v", err)
	}

	// Get list of applied migrations
	rows, err := s.db.Query("SELECT name FROM migrations")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %v", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("failed to scan migration name: %v", err)
		}
		applied[name] = true
	}

	// Get list of migration files
	files, err := filepath.Glob("core/db/V*.sql")
	if err != nil {
		return fmt.Errorf("failed to glob migration files: %v", err)
	}

	// Sort files to ensure correct order
	sort.Strings(files)

	// Run pending migrations
	for _, file := range files {
		name := filepath.Base(file)
		if applied[name] {
			continue
		}

		log.Printf("Running migration: %s", name)
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %v", file, err)
		}

		// Begin transaction
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %v", err)
		}

		// Execute migration
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %v", name, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO migrations (name) VALUES (?)", name); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %v", name, err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %v", name, err)
		}

		log.Printf("Successfully applied migration: %s", name)
	}

	return nil
}

func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("could not connect to database: %v", err)
		log.Fatalf("could not connect to database: %v", err)
		return stats
	}

	dbStats := s.db.Stats()
	stats["status"] = "up"
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["wait_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["wait_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	if dbStats.OpenConnections > 40 {
		stats["message"] = "The database is under heavy load."
	}

	if dbStats.WaitCount > 1000 {
		stats["message"] = "The database has high number of wait events, indicating potential bottlenecks"
	}

	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Many idle connections are being closed, consider revising the connection pool settings."
	}

	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Manye connections are being closed due to max lifetime, consider increasing max lifetime or revising the connection usage pattern."
	}

	return stats
}

func (s *service) Close() error {
	log.Printf("Disconnected from db")
	return s.db.Close()
}
