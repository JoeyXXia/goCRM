package main

import (
	"errors"
	driver "github.com/go-sql-driver/mysql"
	"testing"
)

func expectForeignKeyViolation(t *testing.T, err error) {
	t.Helper()
	var mysqlErr *driver.MySQLError
	if !errors.As(err, &mysqlErr) || (mysqlErr.Number != 1216 && mysqlErr.Number != 1452) {
		t.Fatalf("expected foreign key rejection, got %v", err)
	}
}
