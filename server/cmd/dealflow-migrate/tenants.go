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

//go:embed 002_tenant_memberships.sql
var tenantSQL string

func migrateTenants(ctx context.Context, conn *sql.Conn) error {
	// Check the GVA parent before executing any 002 DDL.
	var parent int
	err := conn.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM information_schema.COLUMNS c
        JOIN information_schema.TABLES t USING (TABLE_SCHEMA, TABLE_NAME)
        WHERE c.TABLE_SCHEMA = DATABASE() AND c.TABLE_NAME = 'sys_users'
          AND c.COLUMN_KEY = 'PRI' AND t.ENGINE = 'InnoDB'
        HAVING COUNT(*) = 1 AND SUM(c.COLUMN_NAME = 'id' AND c.COLUMN_TYPE = 'bigint unsigned') = 1
    `).Scan(&parent)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("migration 002 requires GVA sys_users with a BIGINT UNSIGNED primary key; prepare the GVA user schema in the dedicated test database first")
	}
	if err != nil {
		return fmt.Errorf("inspect GVA user schema: %w", err)
	}

	checksum := sha256.Sum256([]byte(tenantSQL))
	var recorded []byte
	err = conn.QueryRowContext(ctx, "SELECT checksum FROM crm_schema_migrations WHERE version = 2").Scan(&recorded)
	pending := errors.Is(err, sql.ErrNoRows)
	if err != nil && !pending {
		return fmt.Errorf("read migration 002: %w", err)
	}
	if !pending && !bytes.Equal(recorded, checksum[:]) {
		return errors.New("migration 002 checksum mismatch")
	}
	if pending {
		// This embedded file consists only of three independent CREATE TABLE statements.
		for _, statement := range strings.Split(tenantSQL, ";") {
			if statement = strings.TrimSpace(statement); statement != "" {
				if _, err := conn.ExecContext(ctx, statement); err != nil {
					return fmt.Errorf("apply migration 002: %w", err)
				}
			}
		}
	}
	if err := verifyTenantSchema(ctx, conn); err != nil {
		return err
	}
	if pending {
		if _, err := conn.ExecContext(ctx, "INSERT INTO crm_schema_migrations VALUES (2, ?)", checksum[:]); err != nil {
			return fmt.Errorf("record migration 002: %w", err)
		}
	}
	return nil
}

func verifyTenantSchema(ctx context.Context, conn *sql.Conn) error {
	const schemaSQL = `
        SELECT c.TABLE_NAME,
            GROUP_CONCAT(CONCAT(c.COLUMN_NAME, ':', c.COLUMN_TYPE, ':', c.IS_NULLABLE, ':', c.EXTRA)
                ORDER BY c.ORDINAL_POSITION SEPARATOR ',')
        FROM information_schema.COLUMNS c
        JOIN information_schema.TABLES t USING (TABLE_SCHEMA, TABLE_NAME)
        WHERE c.TABLE_SCHEMA = DATABASE() AND t.ENGINE = 'InnoDB'
          AND c.COLUMN_NAME NOT IN ('parent_id', 'permission_version')
          AND c.TABLE_NAME IN ('tenants','departments','tenant_memberships')
        GROUP BY c.TABLE_NAME`
	id := "id:bigint unsigned:NO:auto_increment,"
	expectedColumns := map[string]string{
		"tenants":            id + "code:varchar(64):NO:,name:varchar(128):NO:,status:varchar(16):NO:",
		"departments":        id + "tenant_id:bigint unsigned:NO:,code:varchar(64):NO:,name:varchar(128):NO:",
		"tenant_memberships": id + "tenant_id:bigint unsigned:NO:,user_id:bigint unsigned:NO:,department_id:bigint unsigned:NO:,role_key:varchar(32):NO:,status:varchar(16):NO:",
	}
	if err := verifySignatures(ctx, conn, schemaSQL, expectedColumns); err != nil {
		return err
	}
	const keySQL = `
        SELECT CONCAT(k.TABLE_NAME, '.', k.CONSTRAINT_NAME),
            GROUP_CONCAT(CONCAT(k.COLUMN_NAME,
                IF(k.REFERENCED_TABLE_NAME IS NULL, '',
                   CONCAT('->', k.REFERENCED_TABLE_SCHEMA, '.', k.REFERENCED_TABLE_NAME, '.', k.REFERENCED_COLUMN_NAME)))
                ORDER BY k.ORDINAL_POSITION SEPARATOR ',')
        FROM information_schema.KEY_COLUMN_USAGE k
        WHERE k.TABLE_SCHEMA = DATABASE()
          AND k.CONSTRAINT_NAME <> 'fk_departments_parent'
          AND k.TABLE_NAME IN ('tenants','departments','tenant_memberships')
        GROUP BY k.TABLE_NAME, k.CONSTRAINT_NAME`
	db := testmysql.Database + "."
	return verifySignatures(ctx, conn, keySQL, map[string]string{
		"tenants.PRIMARY":                               "id",
		"tenants.uq_tenants_code":                       "code",
		"departments.PRIMARY":                           "id",
		"departments.uq_departments_tenant_id":          "tenant_id,id",
		"departments.uq_departments_tenant_code":        "tenant_id,code",
		"departments.fk_departments_tenant":             "tenant_id->" + db + "tenants.id",
		"tenant_memberships.PRIMARY":                    "id",
		"tenant_memberships.uq_memberships_tenant_user": "tenant_id,user_id",
		"tenant_memberships.fk_memberships_tenant":      "tenant_id->" + db + "tenants.id",
		"tenant_memberships.fk_memberships_user":        "user_id->" + db + "sys_users.id",
		"tenant_memberships.fk_memberships_department":  "tenant_id->" + db + "departments.tenant_id,department_id->" + db + "departments.id",
	})
}

func verifySignatures(ctx context.Context, conn *sql.Conn, query string, expected map[string]string) error {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("inspect migration 002 schema: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var name, signature string
		if err := rows.Scan(&name, &signature); err != nil {
			return err
		}
		if expected[name] != signature {
			return fmt.Errorf("migration 002 schema mismatch: %s", name)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(expected) {
		return errors.New("migration 002 schema incomplete")
	}
	return nil
}
