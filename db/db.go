package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/aidenappl/openbucket-api/env"

	_ "github.com/go-sql-driver/mysql"
)

const (
	DEFAULT_LIMIT = 50
	MAX_LIMIT     = 500
)

func PingDB(db *sql.DB) error {
	if err := db.Ping(); err != nil {
		fmt.Println(" ❌ Failed")
		return err
	}
	return nil
}

var DB *sql.DB

const schema = "openbucket"

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Init() {
	fmt.Print("Connecting to OpenBucket DB... ")
	dsn := env.CoreDBBase + "/" + schema + "?charset=utf8mb4&parseTime=True&multiStatements=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(fmt.Errorf("error opening database: %w", err))
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	DB = db
}

// RunMigrations executes all SQL migration files in order.
// Uses a migrations_applied table to track which migrations have already been run.
func RunMigrations() {
	fmt.Print("Running migrations... ")

	// Create tracking table
	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS migrations_applied (
		name VARCHAR(255) NOT NULL PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatalf("failed to create migrations table: %v", err)
	}

	// Read migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		log.Fatalf("failed to read migrations directory: %v", err)
	}

	// Sort by name
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	applied := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Check if already applied
		var count int
		err := DB.QueryRow("SELECT COUNT(*) FROM migrations_applied WHERE name = ?", entry.Name()).Scan(&count)
		if err != nil {
			log.Fatalf("failed to check migration status for %s: %v", entry.Name(), err)
		}
		if count > 0 {
			continue
		}

		// Read and execute
		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			log.Fatalf("failed to read migration %s: %v", entry.Name(), err)
		}

		_, err = DB.Exec(string(content))
		if err != nil {
			log.Fatalf("failed to execute migration %s: %v", entry.Name(), err)
		}

		// Record as applied
		_, err = DB.Exec("INSERT INTO migrations_applied (name) VALUES (?)", entry.Name())
		if err != nil {
			log.Fatalf("failed to record migration %s: %v", entry.Name(), err)
		}

		applied++
	}

	if applied > 0 {
		fmt.Printf("✅ %d migration(s) applied\n", applied)
	} else {
		fmt.Println("✅ Up to date")
	}
}

type Queryable interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}
