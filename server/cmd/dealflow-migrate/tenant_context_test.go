package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	router "github.com/flipped-aurora/gin-vue-admin/server/router/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/songzhibin97/gkit/cache/local_cache"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func TestDay4TenantIsolation(t *testing.T) {
	dsn := os.Getenv("DEALFLOW_TEST_DSN")
	if dsn == "" {
		t.Skip("DEALFLOW_TEST_DSN is not set")
	}
	cfg, err := testmysql.Config(dsn)
	requireOK(t, err)
	pool, err := sql.Open("mysql", cfg.FormatDSN())
	requireOK(t, err)
	t.Cleanup(func() { pool.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: pool}), &gorm.Config{
		IgnoreRelationshipsWhenMigrating: true,
		Logger:                           logger.Default.LogMode(logger.Silent),
	})
	requireOK(t, err)
	if !db.Migrator().HasTable(&system.SysUser{}) {
		requireOK(t, db.Migrator().CreateTable(&system.SysUser{}))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	requireOK(t, migrate(ctx, pool, true))

	oldDB, oldConfig, oldCache, oldLog := global.GVA_DB, global.GVA_CONFIG, global.BlackCache, global.GVA_LOG
	t.Cleanup(func() {
		global.GVA_DB, global.GVA_CONFIG, global.BlackCache, global.GVA_LOG = oldDB, oldConfig, oldCache, oldLog
	})
	global.GVA_DB = db
	global.GVA_LOG = zap.NewNop()
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: uuid.NewString(), ExpiresTime: "1h", BufferTime: "1m"}
	global.GVA_CONFIG.System.RouterPrefix = "/api"
	global.BlackCache = local_cache.NewCache(local_cache.SetDefaultExpire(time.Hour))
	enforcer := utils.GetCasbin()
	if enforcer == nil {
		t.Fatal("could not initialize GVA Casbin")
	}
	enforcer.EnableAutoSave(false)
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		rule := []string{"704", "/dealflow/departments/:id", method}
		added, err := enforcer.AddPolicy(rule)
		requireOK(t, err)
		if !added {
			t.Fatal("could not add isolated test policy")
		}
		t.Cleanup(func() {
			_, err := enforcer.RemovePolicy(rule)
			requireOK(t, err)
		})
	}

	tx := db.WithContext(ctx).Begin()
	requireOK(t, tx.Error)
	defer func() { requireOK(t, tx.Rollback().Error) }()
	svc := tenant.New(tx)
	makeFixture := func() (system.SysUser, model.Tenant, model.Department, string) {
		user := system.SysUser{Username: "d4-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1}
		requireOK(t, tx.Omit(clause.Associations).Create(&user).Error)
		enterprise, err := svc.Provision(ctx, tenant.ProvisionInput{
			Code: "d4-" + uuid.NewString(), Name: "Day 4", AdminUserID: uint64(user.ID),
		})
		requireOK(t, err)
		var department model.Department
		requireOK(t, tx.Where("tenant_id = ?", enterprise.ID).Take(&department).Error)
		jwt := utils.NewJWT()
		token, err := jwt.CreateToken(jwt.CreateClaims(request.BaseClaims{ID: user.ID, UUID: user.UUID, AuthorityId: 704}))
		requireOK(t, err)
		return user, enterprise, department, token
	}
	userA, tenantA, deptA, tokenA := makeFixture()
	_, tenantB, deptB, tokenB := makeFixture()
	mux := gin.New()
	router.Register(mux.Group("/api"), func() *gorm.DB { return tx })

	url := func(departmentID uint64) string { return fmt.Sprintf("/api/dealflow/departments/%d", departmentID) }
	call := func(t *testing.T, method, path, token, body string, status int, tenants ...uint64) json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "client-value")
		req.Header.Set("x-token", token)
		for _, id := range tenants {
			req.Header.Add("X-Tenant-ID", fmt.Sprint(id))
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s returned %d, want %d: %s", method, path, rec.Code, status, rec.Body.String())
		}
		if _, err := uuid.Parse(rec.Header().Get("X-Request-ID")); err != nil {
			t.Fatalf("missing server-generated request ID: %v", err)
		}
		var envelope struct {
			Code int
			Data json.RawMessage
			Msg  string
		}
		requireOK(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		if envelope.Msg == "" || len(envelope.Data) == 0 || (envelope.Code == 0) != (status == http.StatusOK) {
			t.Fatalf("unexpected response envelope: %s", rec.Body.String())
		}
		if status >= 400 && string(envelope.Data) != "null" {
			t.Fatalf("error leaked data: %s", rec.Body.String())
		}
		return envelope.Data
	}

	t.Run("read_and_idempotent_rename_own_department", func(t *testing.T) {
		data := call(t, "GET", url(deptA.ID), tokenA, "", 200, tenantA.ID)
		var got model.Department
		requireOK(t, json.Unmarshal(data, &got))
		if got != deptA {
			t.Fatalf("unexpected department: %+v", got)
		}
		for range 2 {
			call(t, "PATCH", url(deptA.ID), tokenA, `{"name":"A renamed"}`, 200, tenantA.ID)
		}
		requireOK(t, tx.Take(&got, deptA.ID).Error)
		if got.Name != "A renamed" || got.TenantID != tenantA.ID || got.Code != deptA.Code {
			t.Fatalf("rename changed unexpected fields: %+v", got)
		}
	})
	t.Run("forged_header_query_body_and_department_id", func(t *testing.T) {
		call(t, "GET", url(deptB.ID), tokenB, "", 200, tenantB.ID)
		call(t, "GET", url(deptB.ID), tokenA, "", 404, tenantA.ID)
		call(t, "PATCH", url(deptB.ID), tokenA, `{"name":"tampered"}`, 404, tenantA.ID)
		call(t, "GET", url(deptB.ID), tokenA, "", 403, tenantB.ID)
		call(t, "PATCH", url(deptB.ID), tokenA, `{"name":"tampered"}`, 403, tenantB.ID)
		call(t, "GET", url(deptB.ID), tokenA, "", 400)
		call(t, "GET", url(deptB.ID), tokenA, "", 400, tenantA.ID, tenantB.ID)
		call(t, "GET", url(deptB.ID)+"?tenant_id="+fmt.Sprint(tenantB.ID), tokenA, "", 400, tenantA.ID)
		call(t, "PATCH", url(deptA.ID), tokenA, fmt.Sprintf(`{"name":"tampered","tenantId":%d}`, tenantB.ID), 400, tenantA.ID)
		call(t, "GET", url(deptB.ID), tokenA+"invalid", "", 401, tenantB.ID)
		var got model.Department
		requireOK(t, tx.Take(&got, deptB.ID).Error)
		if got != deptB {
			t.Fatalf("tenant B changed: before %+v, after %+v", deptB, got)
		}
	})
	t.Run("recheck_membership_tenant_and_user_status", func(t *testing.T) {
		membership := tx.Model(&model.Membership{}).Where("tenant_id = ? AND user_id = ?", tenantA.ID, userA.ID)
		requireOK(t, membership.Update("status", "disabled").Error)
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
		requireOK(t, membership.Update("status", "active").Error)
		requireOK(t, tx.Model(&model.Tenant{}).Where("id = ?", tenantA.ID).Update("status", "disabled").Error)
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
		requireOK(t, tx.Model(&model.Tenant{}).Where("id = ?", tenantA.ID).Update("status", "active").Error)
		requireOK(t, tx.Model(&userA).Update("enable", 2).Error)
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
		requireOK(t, tx.Model(&userA).Update("enable", 1).Error)
		requireOK(t, tx.Create(&model.Membership{
			TenantID: tenantB.ID, UserID: uint64(userA.ID), DepartmentID: deptB.ID, Status: "active", RoleKey: "sales",
		}).Error)
		call(t, "GET", url(deptB.ID), tokenA, "", 200, tenantB.ID)
		call(t, "PATCH", url(deptB.ID), tokenA, `{"name":"wrong role"}`, 403, tenantB.ID)
		requireOK(t, tx.Delete(&userA).Error)
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
	})
}

func TestDay4MissingTenantContext(t *testing.T) {
	// A nil DB makes accidental access fail differently from a missing scope.
	svc := tenant.New(nil)
	ctx := context.Background()
	if _, err := svc.Department(ctx, tenant.TenantContext{}, 1); !errors.Is(err, tenant.ErrTenantDenied) {
		t.Fatalf("unscoped read returned %v", err)
	}
	if err := svc.RenameDepartment(ctx, tenant.TenantContext{}, 1, "new"); !errors.Is(err, tenant.ErrTenantDenied) {
		t.Fatalf("unscoped write returned %v", err)
	}
}
