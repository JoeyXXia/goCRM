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

//go:embed 006_customer_activity.sql
var activitySQL string

func migrateCustomerActivity(ctx context.Context, conn *sql.Conn) error {
	checksum := sha256.Sum256([]byte(activitySQL))
	var recorded []byte
	err := conn.QueryRowContext(ctx, "SELECT checksum FROM crm_schema_migrations WHERE version=6").Scan(&recorded)
	pending := errors.Is(err, sql.ErrNoRows)
	if err != nil && !pending {
		return err
	}
	if !pending && !bytes.Equal(recorded, checksum[:]) {
		return errors.New("migration 006 checksum mismatch")
	}
	if pending {
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='crm_customers' AND COLUMN_NAME='next_followup_at'").Scan(&count); err != nil {
			return err
		}
		for i, statement := range strings.Split(activitySQL, ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" || (i == 0 && count != 0) {
				continue
			}
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
	}
	err = verifySignatures(ctx, conn, `
        SELECT CONCAT(TABLE_NAME,'.',COLUMN_NAME),CONCAT(COLUMN_TYPE,':',IS_NULLABLE)
        FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='crm_customers' AND COLUMN_NAME='next_followup_at'`,
		map[string]string{"crm_customers.next_followup_at": "datetime(3):YES"})
	if err != nil {
		return err
	}
	err = verifySignatures(ctx, conn, `
        SELECT c.TABLE_NAME,GROUP_CONCAT(CONCAT(c.COLUMN_NAME,':',c.COLUMN_TYPE,':',c.IS_NULLABLE,':',c.EXTRA) ORDER BY c.ORDINAL_POSITION)
        FROM information_schema.COLUMNS c JOIN information_schema.TABLES t USING(TABLE_SCHEMA,TABLE_NAME)
        WHERE c.TABLE_SCHEMA=DATABASE() AND t.ENGINE='InnoDB' AND c.TABLE_NAME IN ('crm_followups','crm_audit_events') GROUP BY c.TABLE_NAME`,
		map[string]string{
			"crm_followups":    "id:bigint unsigned:NO:auto_increment,tenant_id:bigint unsigned:NO:,customer_id:bigint unsigned:NO:,actor_id:bigint unsigned:NO:,content:varchar(1024):NO:,created_at:datetime(3):NO:",
			"crm_audit_events": "id:bigint unsigned:NO:auto_increment,tenant_id:bigint unsigned:NO:,customer_id:bigint unsigned:NO:,actor_id:bigint unsigned:NO:,action:varchar(32):NO:,subject_id:bigint unsigned:NO:,from_owner_id:bigint unsigned:NO:,to_owner_id:bigint unsigned:NO:,from_department_id:bigint unsigned:NO:,to_department_id:bigint unsigned:NO:,request_id:varchar(36):NO:,created_at:datetime(3):NO:",
		})
	if err != nil {
		return err
	}
	prefix := testmysql.Database + "."
	err = verifySignatures(ctx, conn, `
        SELECT CONCAT(TABLE_NAME,'.',CONSTRAINT_NAME),GROUP_CONCAT(CONCAT(COLUMN_NAME,
        IF(REFERENCED_TABLE_NAME IS NULL,'',CONCAT('->',REFERENCED_TABLE_SCHEMA,'.',REFERENCED_TABLE_NAME,'.',REFERENCED_COLUMN_NAME))) ORDER BY ORDINAL_POSITION)
        FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME IN ('crm_followups','crm_audit_events') GROUP BY TABLE_NAME,CONSTRAINT_NAME`,
		map[string]string{
			"crm_followups.PRIMARY":               "id",
			"crm_followups.fk_followups_customer": "tenant_id->" + prefix + "crm_customers.tenant_id,customer_id->" + prefix + "crm_customers.id",
			"crm_audit_events.PRIMARY":            "id",
			"crm_audit_events.fk_audit_tenant":    "tenant_id->" + prefix + "tenants.id",
		})
	if err != nil {
		return err
	}
	err = verifySignatures(ctx, conn, `
        SELECT CONCAT(TABLE_NAME,'.',INDEX_NAME),GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX)
        FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
        AND INDEX_NAME IN ('idx_customers_followup','idx_followups_customer','idx_audit_customer')
        GROUP BY TABLE_NAME,INDEX_NAME`, map[string]string{
		"crm_customers.idx_customers_followup": "tenant_id,next_followup_at,id",
		"crm_followups.idx_followups_customer": "tenant_id,customer_id,created_at,id",
		"crm_audit_events.idx_audit_customer":  "tenant_id,customer_id,created_at,id",
	})
	if err != nil {
		return err
	}
	if pending {
		_, err = conn.ExecContext(ctx, "INSERT INTO crm_schema_migrations VALUES (6, ?)", checksum[:])
	}
	return err
}
