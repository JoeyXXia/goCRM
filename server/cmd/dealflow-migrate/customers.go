package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"errors"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
)

//go:embed 005_customers.sql
var customerSQL string

func migrateCustomers(ctx context.Context, conn *sql.Conn) error {
	checksum := sha256.Sum256([]byte(customerSQL))
	var recorded []byte
	err := conn.QueryRowContext(ctx, "SELECT checksum FROM crm_schema_migrations WHERE version=5").Scan(&recorded)
	pending := errors.Is(err, sql.ErrNoRows)
	if err != nil && !pending {
		return err
	}
	if !pending && !bytes.Equal(recorded, checksum[:]) {
		return errors.New("migration 005 checksum mismatch")
	}
	if pending {
		for _, statement := range strings.Split(customerSQL, ";") {
			if statement = strings.TrimSpace(statement); statement != "" {
				if _, err := conn.ExecContext(ctx, statement); err != nil {
					return err
				}
			}
		}
	}
	err = verifySignatures(ctx, conn, `
        SELECT c.TABLE_NAME, GROUP_CONCAT(CONCAT(c.COLUMN_NAME,':',c.COLUMN_TYPE,':',c.IS_NULLABLE,':',c.EXTRA) ORDER BY c.ORDINAL_POSITION)
        FROM information_schema.COLUMNS c JOIN information_schema.TABLES t USING(TABLE_SCHEMA,TABLE_NAME)
        WHERE c.TABLE_SCHEMA=DATABASE() AND t.ENGINE='InnoDB' AND c.TABLE_NAME IN ('crm_customers','crm_contacts')
        GROUP BY c.TABLE_NAME`, map[string]string{
		"crm_customers": "id:bigint unsigned:NO:auto_increment,tenant_id:bigint unsigned:NO:,code:varchar(38):NO:,name:varchar(128):NO:,owner_id:bigint unsigned:NO:,department_id:bigint unsigned:NO:,industry:varchar(64):NO:,remark:varchar(512):NO:",
		"crm_contacts":  "id:bigint unsigned:NO:auto_increment,tenant_id:bigint unsigned:NO:,customer_id:bigint unsigned:NO:,name:varchar(128):NO:,phone:varchar(32):NO:,email:varchar(254):NO:,title:varchar(64):NO:",
	})
	if err != nil {
		return err
	}
	prefix := testmysql.Database + "."
	err = verifySignatures(ctx, conn, `
        SELECT CONCAT(TABLE_NAME,'.',CONSTRAINT_NAME), GROUP_CONCAT(CONCAT(COLUMN_NAME,
          IF(REFERENCED_TABLE_NAME IS NULL,'',CONCAT('->',REFERENCED_TABLE_SCHEMA,'.',REFERENCED_TABLE_NAME,'.',REFERENCED_COLUMN_NAME))) ORDER BY ORDINAL_POSITION)
        FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('crm_customers','crm_contacts')
        GROUP BY TABLE_NAME,CONSTRAINT_NAME`, map[string]string{
		"crm_customers.PRIMARY":                  "id",
		"crm_customers.uq_customers_tenant_id":   "tenant_id,id",
		"crm_customers.uq_customers_tenant_code": "tenant_id,code",
		"crm_customers.fk_customers_tenant":      "tenant_id->" + prefix + "tenants.id",
		"crm_customers.fk_customers_owner":       "tenant_id->" + prefix + "tenant_memberships.tenant_id,owner_id->" + prefix + "tenant_memberships.user_id",
		"crm_customers.fk_customers_department":  "tenant_id->" + prefix + "departments.tenant_id,department_id->" + prefix + "departments.id",
		"crm_contacts.PRIMARY":                   "id",
		"crm_contacts.fk_contacts_customer":      "tenant_id->" + prefix + "crm_customers.tenant_id,customer_id->" + prefix + "crm_customers.id",
	})
	if err != nil {
		return err
	}
	if pending {
		_, err = conn.ExecContext(ctx, "INSERT INTO crm_schema_migrations VALUES (5, ?)", checksum[:])
	}
	return err
}
