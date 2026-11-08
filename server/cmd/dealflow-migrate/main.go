package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

const testDatabase = "dealflow_crm_test"
const lockName = "dealflow_crm_test:migrate"

//go:embed 001_schema_migrations.sql
var baselineSQL string

func testConfig(dsn string) (*mysql.Config, error) {
	if dsn == "" {
		return nil, errors.New("set DEALFLOW_TEST_DSN")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, errors.New("invalid DEALFLOW_TEST_DSN format")
	}
	if cfg.DBName != testDatabase {
		return nil, fmt.Errorf("DEALFLOW_TEST_DSN must select %s", testDatabase)
	}
	return cfg, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	// The named lock belongs to the connection, so all steps use this session.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect to test MySQL: %w", err)
	}
	defer conn.Close()

	var database string
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
		return fmt.Errorf("verify selected test database: %w", err)
	}
	if database != testDatabase {
		return fmt.Errorf("selected database must be %s", testDatabase)
	}

	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 5)", lockName).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return errors.New("could not acquire migration lock")
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(releaseCtx, "DO RELEASE_LOCK(?)", lockName); err != nil {
			// Discard the session rather than return a possibly locked session to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()

	if _, err := conn.ExecContext(ctx, baselineSQL); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	if err := verifyBaseline(ctx, conn); err != nil {
		return err
	}

	checksum := sha256.Sum256([]byte(baselineSQL))
	var recorded []byte
	err = conn.QueryRowContext(ctx, "SELECT checksum FROM crm_schema_migrations WHERE version = 1").Scan(&recorded)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := conn.ExecContext(ctx, "INSERT INTO crm_schema_migrations (version, checksum) VALUES (1, ?)", checksum[:]); err != nil {
			return fmt.Errorf("record migration 001: %w", err)
		}
	case err != nil:
		return fmt.Errorf("read migration 001: %w", err)
	case !bytes.Equal(recorded, checksum[:]):
		return errors.New("migration 001 checksum mismatch")
	}
	return nil
}

func verifyBaseline(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, `
        SELECT c.COLUMN_NAME, c.COLUMN_TYPE, c.IS_NULLABLE, c.COLUMN_KEY, t.ENGINE
        FROM information_schema.COLUMNS c
        JOIN information_schema.TABLES t
          ON t.TABLE_SCHEMA = c.TABLE_SCHEMA AND t.TABLE_NAME = c.TABLE_NAME
        WHERE c.TABLE_SCHEMA = DATABASE() AND c.TABLE_NAME = 'crm_schema_migrations'
        ORDER BY c.ORDINAL_POSITION`)
	if err != nil {
		return fmt.Errorf("inspect migration table: %w", err)
	}
	defer rows.Close()

	expected := [][5]string{
		{"version", "int unsigned", "NO", "PRI", "InnoDB"},
		{"checksum", "binary(32)", "NO", "", "InnoDB"},
	}
	count := 0
	for rows.Next() {
		var column [5]string
		if err := rows.Scan(&column[0], &column[1], &column[2], &column[3], &column[4]); err != nil {
			return fmt.Errorf("inspect migration table column: %w", err)
		}
		if count >= len(expected) || column != expected[count] {
			return errors.New("migration table structure mismatch")
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect migration table: %w", err)
	}
	if count != len(expected) {
		return errors.New("migration table structure mismatch")
	}
	return nil
}

func main() {
	cfg, err := testConfig(os.Getenv("DEALFLOW_TEST_DSN"))
	if err != nil {
		log.Fatal(err)
	}
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		log.Fatal("invalid test MySQL connection configuration")
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := migrate(ctx, db); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Migration 001 verified")
}
