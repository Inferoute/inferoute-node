package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
	_ "github.com/lib/pq"
)

// DB represents our database connection pool
type DB struct {
	*sql.DB
}

// Config holds database configuration
type Config struct {
	Host     string `mapstructure:"database_host"`
	Port     int    `mapstructure:"database_port"`
	User     string `mapstructure:"database_user"`
	Password string `mapstructure:"database_password"`
	DBName   string `mapstructure:"database_dbname"`
	SSLMode  string `mapstructure:"database_sslmode"`
}

// New creates a new database connection pool
func New(host string, port int, user, password, dbname, sslmode string) (*DB, error) {
	connStr := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s&search_path=public",
		user, password, host, port, dbname, sslmode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	return &DB{db}, nil
}

// ExecuteTx executes a function within a transaction with retries
func (db *DB) ExecuteTx(ctx context.Context, fn func(*sql.Tx) error) error {
	return crdb.ExecuteTx(ctx, db.DB, nil, fn)
}

// Close closes the database connection pool
func (db *DB) Close() error {
	return db.DB.Close()
}

// WithTransaction executes the given function within a transaction
func (db *DB) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			// A panic occurred, rollback and repanic
			tx.Rollback()
			panic(p)
		} else if err != nil {
			// Something went wrong, rollback
			tx.Rollback()
		} else {
			// All good, commit
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// HealthCheck performs a health check on the database
func (db *DB) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("Database health check failed: %v", err)
		return err
	}

	return nil
}

// TxFuncInt is a function that executes in a transaction and returns an int and error
type TxFuncInt func(*sql.Tx) (int, error)

// ExecuteTxInt executes f in a transaction and returns an int
func (db *DB) ExecuteTxInt(ctx context.Context, f TxFuncInt) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %w", err)
	}

	result, err := f(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return 0, fmt.Errorf("error rolling back transaction: %v (original error: %w)", rbErr, err)
		}
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %w", err)
	}

	return result, nil
}
