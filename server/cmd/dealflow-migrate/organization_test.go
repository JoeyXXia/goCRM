package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func TestDay8ConcurrentOrganization(t *testing.T) {
	dsn := os.Getenv("DEALFLOW_TEST_DSN")
	if dsn == "" {
		t.Skip("DEALFLOW_TEST_DSN is not set")
	}
	cfg, err := testmysql.Config(dsn)
	requireOK(t, err)
	cfg.ParseTime = true
	pool, err := sql.Open("mysql", cfg.FormatDSN())
	requireOK(t, err)
	t.Cleanup(func() { pool.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: pool}), &gorm.Config{IgnoreRelationshipsWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	requireOK(t, err)
	if !db.Migrator().HasTable(&system.SysUser{}) {
		requireOK(t, db.Migrator().CreateTable(&system.SysUser{}))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	requireOK(t, migrate(ctx, pool, true))
	requireOK(t, migrate(ctx, pool, true))
	users := []system.SysUser{
		{Username: "d8-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1},
		{Username: "d8-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1},
	}
	requireOK(t, db.Omit(clause.Associations).Create(&users).Error)
	t.Cleanup(func() {
		requireOK(t, db.Where("id IN ?", []uint{users[0].ID, users[1].ID}).Unscoped().Delete(&system.SysUser{}).Error)
	})
	svc := tenant.New(db)
	enterprise, err := svc.Provision(ctx, tenant.ProvisionInput{Code: "d8-" + uuid.NewString(), Name: "Concurrency", AdminUserID: uint64(users[0].ID)})
	requireOK(t, err)
	t.Cleanup(func() {
		requireOK(t, db.Where("tenant_id = ?", enterprise.ID).Delete(&model.Membership{}).Error)
		requireOK(t, db.Model(&model.Department{}).Where("tenant_id = ?", enterprise.ID).Update("parent_id", nil).Error)
		requireOK(t, db.Where("tenant_id = ?", enterprise.ID).Delete(&model.Department{}).Error)
		requireOK(t, db.Delete(&model.Tenant{}, enterprise.ID).Error)
	})
	var dept model.Department
	requireOK(t, db.Where("tenant_id = ?", enterprise.ID).Take(&dept).Error)
	scope, err := svc.ResolveTenant(ctx, uint64(users[0].ID), enterprise.ID)
	requireOK(t, err)
	second, err := svc.SaveMember(ctx, scope, 0, tenant.MemberInput{UserID: uint64(users[1].ID), DepartmentID: dept.ID, RoleKey: "tenant_admin", Status: "active"})
	requireOK(t, err)
	scopeB, err := svc.ResolveTenant(ctx, uint64(users[1].ID), enterprise.ID)
	requireOK(t, err)
	var first model.Membership
	requireOK(t, db.Where("tenant_id = ? AND user_id = ?", enterprise.ID, users[0].ID).Take(&first).Error)

	runConcurrently := func(a, b func() error) {
		t.Helper()
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, work := range []func() error{a, b} {
			wg.Add(1)
			go func(work func() error) { defer wg.Done(); <-start; results <- work() }(work)
		}
		close(start)
		wg.Wait()
		close(results)
		successes, conflicts := 0, 0
		for err := range results {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, tenant.ErrOrganizationConflict):
				conflicts++
			default:
				t.Fatalf("unexpected concurrent result: %v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("success=%d conflict=%d", successes, conflicts)
		}
	}
	left, err := svc.SaveDepartment(ctx, scope, 0, tenant.DepartmentInput{Code: "left", Name: "Left"})
	requireOK(t, err)
	right, err := svc.SaveDepartment(ctx, scope, 0, tenant.DepartmentInput{Code: "right", Name: "Right"})
	requireOK(t, err)
	runConcurrently(func() error {
		_, err := svc.SaveDepartment(ctx, scope, left.ID, tenant.DepartmentInput{Code: "left", Name: "Left", ParentID: &right.ID})
		return err
	}, func() error {
		_, err := svc.SaveDepartment(ctx, scopeB, right.ID, tenant.DepartmentInput{Code: "right", Name: "Right", ParentID: &left.ID})
		return err
	})
	requireOK(t, db.Take(&left, left.ID).Error)
	requireOK(t, db.Take(&right, right.ID).Error)
	if (left.ParentID == nil) == (right.ParentID == nil) {
		t.Fatal("concurrent moves must leave one root")
	}
	runConcurrently(func() error {
		_, err := svc.SaveMember(ctx, scope, first.ID, tenant.MemberInput{UserID: first.UserID, DepartmentID: dept.ID, RoleKey: "tenant_admin", Status: "disabled"})
		return err
	}, func() error {
		_, err := svc.SaveMember(ctx, scopeB, second.ID, tenant.MemberInput{UserID: second.UserID, DepartmentID: dept.ID, RoleKey: "tenant_admin", Status: "disabled"})
		return err
	})
	var count int64
	requireOK(t, db.Model(&model.Membership{}).Where("tenant_id = ? AND role_key = ? AND status = ?", enterprise.ID, "tenant_admin", "active").Count(&count).Error)
	if count != 1 {
		t.Fatalf("expected exactly one active administrator, got %d", count)
	}
}
