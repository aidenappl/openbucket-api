package db

import (
	"database/sql"
	"fmt"
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

func Init() {
	fmt.Print("Connecting to OpenBucket DB... ")
	dsn := env.CoreDBBase + "/" + schema + "?charset=utf8mb4&parseTime=True"
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

type Queryable interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}
