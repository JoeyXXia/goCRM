package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
)

func TestRejectNonTestDatabase(t *testing.T) {
	for _, dsn := range []string{"", "invalid", "user@tcp(127.0.0.1:3306)/", "user@tcp(127.0.0.1:3306)/dealflow_crm_day1"} {
		if _, err := testmysql.Config(dsn); err == nil {
			t.Fatalf("accepted DSN with missing or invalid test database: %q", dsn)
		}
	}
}

func TestMySQLMigrationAndRollback(t *testing.T) {
	dsn := os.Getenv("DEALFLOW_TEST_DSN")
	if dsn == "" {
		t.Skip("set DEALFLOW_TEST_DSN to run MySQL integration")
	}
	cfg, err := testmysql.Config(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("could not open test MySQL")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < 2; i++ {
		if err := migrate(ctx, db, false); err != nil {
			t.Fatal(err)
		}
	}
	var checksum []byte
	if err := db.QueryRowContext(ctx, "SELECT checksum FROM crm_schema_migrations WHERE version = 1").Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(baselineSQL))
	if !bytes.Equal(checksum, expected[:]) {
		t.Fatal("incorrect recorded checksum")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "INSERT INTO crm_schema_migrations VALUES (4294967295, ?)", checksum); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM crm_schema_migrations WHERE version = 4294967295").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("test fixture survived rollback")
	}
}
