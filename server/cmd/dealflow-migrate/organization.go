package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
)

//go:embed 003_organization.sql
var organizationSQL string

func migrateOrganization(ctx context.Context, conn *sql.Conn) error {
	checksum := sha256.Sum256([]byte(organizationSQL))
	var recorded []byte
	err := conn.QueryRowContext(ctx, "SELECT checksum FROM crm_schema_migrations WHERE version = 3").Scan(&recorded)
	pending := errors.Is(err, sql.ErrNoRows)
	if err != nil && !pending {
		return err
	}
	if !pending && !bytes.Equal(recorded, checksum[:]) {
		return errors.New("migration 003 checksum mismatch")
	}
	if pending {
		// MySQL DDL commits independently. Resume only a missing atomic ALTER.
		for i, statement := range strings.Split(organizationSQL, ";")[:2] {
			table, column := "departments", "parent_id"
			if i == 1 {
				table, column = "tenant_memberships", "permission_version"
			}
			var count int
			err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, column).Scan(&count)
			if err != nil {
				return err
			}
			if count == 0 {
				if _, err := conn.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("apply migration 003: %w", err)
				}
			}
		}
	}
	err = verifySignatures(ctx, conn, `
        SELECT CONCAT(TABLE_NAME,'.',COLUMN_NAME), CONCAT(COLUMN_TYPE,':',IS_NULLABLE,':',COALESCE(COLUMN_DEFAULT,'NULL'))
        FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
        AND ((TABLE_NAME='departments' AND COLUMN_NAME='parent_id')
          OR (TABLE_NAME='tenant_memberships' AND COLUMN_NAME='permission_version'))`, map[string]string{
		"departments.parent_id":                 "bigint unsigned:YES:NULL",
		"tenant_memberships.permission_version": "bigint unsigned:NO:1",
	})
	if err != nil {
		return err
	}
	err = verifySignatures(ctx, conn, `
        SELECT CONSTRAINT_NAME, GROUP_CONCAT(CONCAT(COLUMN_NAME,'->',REFERENCED_TABLE_SCHEMA,'.',REFERENCED_TABLE_NAME,'.',REFERENCED_COLUMN_NAME) ORDER BY ORDINAL_POSITION)
        FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='departments'
        AND CONSTRAINT_NAME='fk_departments_parent' GROUP BY CONSTRAINT_NAME`, map[string]string{
		"fk_departments_parent": "tenant_id->" + testmysql.Database + ".departments.tenant_id,parent_id->" + testmysql.Database + ".departments.id",
	})
	if err != nil {
		return err
	}
	if pending {
		_, err = conn.ExecContext(ctx, "INSERT INTO crm_schema_migrations VALUES (3, ?)", checksum[:])
	}
	return err
}
