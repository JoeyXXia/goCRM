package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"errors"
)

//go:embed 004_data_roles.sql
var dataRolesSQL string

func migrateDataRoles(ctx context.Context, conn *sql.Conn) error {
	checksum := sha256.Sum256([]byte(dataRolesSQL))
	var recorded []byte
	err := conn.QueryRowContext(ctx, "SELECT checksum FROM crm_schema_migrations WHERE version = 4").Scan(&recorded)
	pending := errors.Is(err, sql.ErrNoRows)
	if err != nil && !pending {
		return err
	}
	if !pending && !bytes.Equal(recorded, checksum[:]) {
		return errors.New("migration 004 checksum mismatch")
	}
	if pending {
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='tenant_memberships' AND COLUMN_NAME='data_roles'").Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := conn.ExecContext(ctx, dataRolesSQL); err != nil {
				return err
			}
		}
	}
	if err := verifySignatures(ctx, conn, `
        SELECT COLUMN_NAME, CONCAT(COLUMN_TYPE,':',IS_NULLABLE,':',COALESCE(COLUMN_DEFAULT,'NULL'))
        FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='tenant_memberships' AND COLUMN_NAME='data_roles'`,
		map[string]string{"data_roles": "json:YES:NULL"}); err != nil {
		return err
	}
	if pending {
		_, err = conn.ExecContext(ctx, "INSERT INTO crm_schema_migrations VALUES (4, ?)", checksum[:])
	}
	return err
}
