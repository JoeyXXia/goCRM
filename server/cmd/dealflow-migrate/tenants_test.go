package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	driver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func TestDay3TenantLifecycle(t *testing.T) {
	dsn := os.Getenv("DEALFLOW_TEST_DSN")
	if dsn == "" {
		t.Skip("DEALFLOW_TEST_DSN is not set")
	}
	cfg, err := testmysql.Config(dsn)
	requireOK(t, err)
	pool, err := sql.Open("mysql", cfg.FormatDSN())
	requireOK(t, err)
	t.Cleanup(func() { pool.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: pool}), &gorm.Config{
		Logger:                           logger.Default.LogMode(logger.Silent),
		IgnoreRelationshipsWhenMigrating: true,
	})
	requireOK(t, err)
	if !db.Migrator().HasTable(&system.SysUser{}) {
		err := migrate(ctx, pool, true)
		if err == nil || !strings.Contains(err.Error(), "requires GVA sys_users") {
			t.Fatalf("expected GVA schema preflight failure, got %v", err)
		}
		for _, table := range []string{"tenants", "departments", "tenant_memberships"} {
			if db.Migrator().HasTable(table) {
				t.Fatalf("created %s without GVA parent", table)
			}
		}
		requireOK(t, db.Migrator().CreateTable(&system.SysUser{}))
	}
	requireOK(t, migrate(ctx, pool, true))
	requireOK(t, migrate(ctx, pool, true))

	newUser := func(enabled int, deleted bool) uint64 {
		id := uuid.New()
		user := system.SysUser{Username: "d3-" + id.String(), UUID: id, Enable: enabled, Password: "!"}
		if deleted {
			user.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		}
		requireOK(t, db.Omit(clause.Associations).Create(&user).Error)
		t.Cleanup(func() { requireOK(t, db.Exec("DELETE FROM sys_users WHERE id = ?", user.ID).Error) })
		return uint64(user.ID)
	}
	demo := tenant.DemoUsers{Admin: newUser(1, false), Sales: newUser(1, false), Manager: newUser(1, false), Finance: newUser(1, false)}
	disabled := newUser(2, false)
	deleted := newUser(1, true)
	code := "d3-" + uuid.NewString()
	codes := []string{"demo-a", "demo-b", code, code + "-rollback"}
	var existing int64
	requireOK(t, db.Model(&model.Tenant{}).Where("code IN ?", codes).Count(&existing).Error)
	if existing != 0 {
		t.Fatal("use a test database without existing demo tenant codes")
	}
	t.Cleanup(func() {
		ids := db.Model(&model.Tenant{}).Select("id").Where("code IN ?", codes)
		requireOK(t, db.Where("tenant_id IN (?)", ids).Delete(&model.Membership{}).Error)
		requireOK(t, db.Where("tenant_id IN (?)", ids).Delete(&model.Department{}).Error)
		requireOK(t, db.Where("code IN ?", codes).Delete(&model.Tenant{}).Error)
	})
	svc := tenant.New(db)

	t.Run("seed_and_foreign_keys", func(t *testing.T) {
		requireOK(t, svc.SeedDemo(ctx, demo))
		var a, b model.Tenant
		requireOK(t, db.Where("code = ?", "demo-a").Take(&a).Error)
		requireOK(t, db.Where("code = ?", "demo-b").Take(&b).Error)
		read := func() []model.Membership {
			var rows []model.Membership
			requireOK(t, db.Where("tenant_id IN ?", []uint64{a.ID, b.ID}).Order("id").Find(&rows).Error)
			return rows
		}
		before := read()
		requireOK(t, svc.SeedDemo(ctx, demo))
		if len(before) != 8 || !reflect.DeepEqual(before, read()) {
			t.Fatal("seed retry did not preserve exactly eight memberships")
		}
		var salesA, salesB model.Membership
		requireOK(t, db.Where("tenant_id = ? AND user_id = ?", a.ID, demo.Sales).Take(&salesA).Error)
		requireOK(t, db.Where("tenant_id = ? AND user_id = ?", b.ID, demo.Sales).Take(&salesB).Error)
		if salesA.RoleKey != "sales" || salesB.RoleKey != "manager" {
			t.Fatalf("same user has unexpected tenant roles: %s / %s", salesA.RoleKey, salesB.RoleKey)
		}
		changed := demo
		changed.Sales, changed.Manager = changed.Manager, changed.Sales
		if err := svc.SeedDemo(ctx, changed); err == nil || !reflect.DeepEqual(before, read()) {
			t.Fatal("conflicting seed must fail and preserve the original memberships")
		}
		deptA := salesA.DepartmentID
		salesA.ID = 0
		expectMySQLCode(t, db.Create(&salesA).Error, 1062)
		salesA.UserID = disabled
		salesA.DepartmentID = salesB.DepartmentID
		rejectCrossTenantDepartment := func(err error) {
			t.Helper()
			var fkErr *driver.MySQLError
			// MySQL can omit constraint details for restricted accounts (1216).
			if !errors.As(err, &fkErr) || (fkErr.Number != 1216 && fkErr.Number != 1452) {
				t.Fatalf("expected MySQL foreign key violation, got %v", err)
			}
		}
		rejectCrossTenantDepartment(db.Create(&salesA).Error)
		rejectCrossTenantDepartment(db.Model(&salesB).Update("department_id", deptA).Error)
		if !reflect.DeepEqual(before, read()) {
			t.Fatal("cross-tenant insert or update changed memberships")
		}
	})

	t.Run("provision_admin", func(t *testing.T) {
		input := tenant.ProvisionInput{Code: code, Name: "Day 3 tenant", AdminUserID: disabled}
		for _, id := range []uint64{disabled, deleted} {
			input.AdminUserID = id
			if _, err := svc.Provision(ctx, input); err == nil || !strings.Contains(err.Error(), "missing, disabled or deleted") {
				t.Fatalf("ineligible GVA account accepted: %v", err)
			}
		}
		input.AdminUserID = demo.Admin
		created, err := svc.Provision(ctx, input)
		requireOK(t, err)
		var membership model.Membership
		requireOK(t, db.Where("tenant_id = ?", created.ID).Take(&membership).Error)
		var department model.Department
		requireOK(t, db.Take(&department, membership.DepartmentID).Error)
		if membership.UserID != demo.Admin || membership.RoleKey != "tenant_admin" ||
			membership.Status != "active" || department.TenantID != created.ID || department.Code != "general" {
			t.Fatal("first administrator and default department were not linked")
		}
		_, err = svc.Provision(ctx, input)
		expectMySQLCode(t, err, 1062)
	})

	t.Run("rollback_after_department_insert", func(t *testing.T) {
		faulty, err := gorm.Open(mysql.New(mysql.Config{Conn: pool}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		requireOK(t, err)
		injected := errors.New("test failure at membership insert")
		var tenantID, departmentID uint64
		requireOK(t, faulty.Callback().Create().Before("gorm:create").Register("test:fail_membership", func(tx *gorm.DB) {
			if tx.Statement.Table != "tenant_memberships" {
				return
			}
			binding := tx.Statement.Dest.(*model.Membership)
			row := tx.Statement.ConnPool.QueryRowContext(ctx, "SELECT id FROM departments WHERE id = ? AND tenant_id = ?", binding.DepartmentID, binding.TenantID)
			if err := row.Scan(&departmentID); err != nil {
				tx.AddError(err)
				return
			}
			tenantID = binding.TenantID
			tx.AddError(injected)
		}))
		created, err := tenant.New(faulty).Provision(ctx, tenant.ProvisionInput{Code: code + "-rollback", Name: "Rollback", AdminUserID: demo.Admin})
		if !errors.Is(err, injected) || created.ID != 0 || tenantID == 0 || departmentID == 0 {
			t.Fatalf("failure was not injected after parent writes: %v", err)
		}
		for _, query := range []string{
			"SELECT COUNT(*) FROM tenants WHERE id = ?",
			"SELECT COUNT(*) FROM departments WHERE tenant_id = ?",
			"SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id = ?",
		} {
			var count int
			requireOK(t, pool.QueryRowContext(ctx, query, tenantID).Scan(&count))
			if count != 0 {
				t.Fatalf("transaction left %d rows for %s", count, query)
			}
		}
	})
}

func requireOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func expectMySQLCode(t *testing.T, err error, code uint16) {
	t.Helper()
	var mysqlErr *driver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != code {
		t.Fatalf("expected MySQL %d, got %v", code, err)
	}
}
