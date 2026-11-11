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
	"slices"
	"strings"
	"testing"
	"time"

	tenantapi "github.com/flipped-aurora/gin-vue-admin/server/api/v1/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	examplemodel "github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	examplerouter "github.com/flipped-aurora/gin-vue-admin/server/router/example"
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

func TestDay4And5AccessIsolation(t *testing.T) {
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
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: pool}), &gorm.Config{
		IgnoreRelationshipsWhenMigrating: true,
		Logger:                           logger.Default.LogMode(logger.Silent),
	})
	requireOK(t, err)
	if !db.Migrator().HasTable(&system.SysUser{}) {
		requireOK(t, db.Migrator().CreateTable(&system.SysUser{}))
	}
	if !db.Migrator().HasTable(&examplemodel.ExaCustomer{}) {
		requireOK(t, db.Migrator().CreateTable(&examplemodel.ExaCustomer{}))
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
	for _, path := range []string{"/customer/customer", "/customer/customerList"} {
		rule := []string{"704", path, http.MethodGet}
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
	userB, tenantB, deptB, tokenB := makeFixture()
	operator, operatorTenant, operatorDept, operatorToken := makeFixture()
	mux := gin.New()
	platform := tenantapi.PlatformAPI{Operator: operator.UUID, DB: func() *gorm.DB { return tx }}
	router.Register(mux.Group("/api"), platform)
	upstream := mux.Group("/api", tenantapi.RequestID, middleware.JWTAuth(), platform.BlockOperator, middleware.CasbinHandler())
	(&examplerouter.CustomerRouter{}).InitCustomerRouter(upstream)

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
	const workspaces = "/api/dealflow/workspaces"
	t.Run("day8_tree_and_member_lifecycle", func(t *testing.T) {
		base := "/api/dealflow"
		var child, leaf model.Department
		data := call(t, "POST", base+"/departments", tokenA,
			fmt.Sprintf(`{"code":"child","name":"同名部门","parentId":"%d"}`, deptA.ID), 200, tenantA.ID)
		requireOK(t, json.Unmarshal(data, &child))
		data = call(t, "POST", base+"/departments", tokenA,
			fmt.Sprintf(`{"code":"leaf","name":"同名部门","parentId":"%d"}`, child.ID), 200, tenantA.ID)
		requireOK(t, json.Unmarshal(data, &leaf))
		call(t, "POST", base+"/departments", tokenB, `{"code":"child","name":"同名部门"}`, 200, tenantB.ID)
		call(t, "POST", base+"/departments", tokenA, `{"code":"child","name":"duplicate"}`, 409, tenantA.ID)
		call(t, "POST", base+"/departments", tokenA, fmt.Sprintf(`{"code":"foreign","name":"bad","parentId":"%d"}`, deptB.ID), 404, tenantA.ID)
		call(t, "PUT", url(deptA.ID), tokenA, fmt.Sprintf(`{"code":"general","name":"bad","parentId":"%d"}`, leaf.ID), 409, tenantA.ID)
		call(t, "PUT", url(child.ID), tokenA, fmt.Sprintf(`{"code":"child","name":"bad","parentId":"%d"}`, child.ID), 409, tenantA.ID)
		var parents []model.Department
		requireOK(t, json.Unmarshal(call(t, "GET", url(leaf.ID)+"/ancestors", tokenA, "", 200, tenantA.ID), &parents))
		if len(parents) != 2 || parents[0].ID != child.ID || parents[1].ID != deptA.ID {
			t.Fatal("wrong ancestor chain")
		}
		call(t, "GET", base+"/departments", tokenA, "", 200, tenantA.ID)
		call(t, "GET", url(leaf.ID)+"/ancestors", tokenB, "", 404, tenantB.ID)
		call(t, "DELETE", url(child.ID), tokenA, "", 409, tenantA.ID)
		var member model.Membership
		input := func(dept uint64, role, status string) string {
			return fmt.Sprintf(`{"userId":"%d","departmentId":"%d","roleKey":"%s","status":"%s"}`, userB.ID, dept, role, status)
		}
		data = call(t, "POST", base+"/members", tokenA, input(leaf.ID, "sales", "active"), 200, tenantA.ID)
		requireOK(t, json.Unmarshal(data, &member))
		if member.PermissionVersion != 1 {
			t.Fatal("new member version must be one")
		}
		memberURL := fmt.Sprintf("%s/members/%d", base, member.ID)
		call(t, "POST", base+"/members", tokenA, input(leaf.ID, "sales", "active"), 409, tenantA.ID)
		call(t, "PUT", memberURL, tokenB, input(leaf.ID, "sales", "active"), 403, tenantA.ID)
		call(t, "PUT", memberURL, tokenA, input(deptB.ID, "sales", "active"), 404, tenantA.ID)
		call(t, "DELETE", url(leaf.ID), tokenA, "", 409, tenantA.ID)
		var admin model.Membership
		requireOK(t, tx.Where("tenant_id = ? AND user_id = ?", tenantA.ID, userA.ID).Take(&admin).Error)
		adminURL := fmt.Sprintf("%s/members/%d", base, admin.ID)
		call(t, "DELETE", adminURL, tokenA, "", 409, tenantA.ID)
		for _, role := range []string{"sales", "tenant_admin"} {
			status := "active"
			if role == "tenant_admin" {
				status = "disabled"
			}
			body := fmt.Sprintf(`{"userId":"%d","departmentId":"%d","roleKey":"%s","status":"%s"}`, userA.ID, deptA.ID, role, status)
			call(t, "PUT", adminURL, tokenA, body, 409, tenantA.ID)
		}
		call(t, "PUT", memberURL, tokenA, input(child.ID, "tenant_admin", "active"), 200, tenantA.ID)
		call(t, "GET", base+"/members", tokenB, "", 200, tenantA.ID)
		data = call(t, "PUT", memberURL, tokenA, input(child.ID, "sales", "disabled"), 200, tenantA.ID)
		requireOK(t, json.Unmarshal(data, &member))
		if member.PermissionVersion != 3 {
			t.Fatalf("expected version three, got %d", member.PermissionVersion)
		}
		requireOK(t, tx.Model(&system.SysUser{}).Where("id = ?", userB.ID).Update("enable", 2).Error)
		data = call(t, "PUT", memberURL, tokenA, input(child.ID, "sales", "disabled"), 200, tenantA.ID)
		requireOK(t, json.Unmarshal(data, &member))
		if member.PermissionVersion != 3 {
			t.Fatal("idempotent update incremented permission version")
		}
		requireOK(t, tx.Model(&system.SysUser{}).Where("id = ?", userB.ID).Update("enable", 1).Error)
		call(t, "GET", url(child.ID), tokenB, "", 403, tenantA.ID)
		call(t, "GET", base+"/members", tokenB, "", 403, tenantA.ID)
		call(t, "PUT", memberURL, tokenA, input(child.ID, "sales", "active"), 200, tenantA.ID)
		call(t, "GET", url(child.ID), tokenB, "", 200, tenantA.ID)
		call(t, "DELETE", memberURL, tokenA, "", 200, tenantA.ID)
		call(t, "GET", url(child.ID), tokenB, "", 403, tenantA.ID)
		call(t, "DELETE", url(leaf.ID), tokenA, "", 200, tenantA.ID)
		call(t, "DELETE", url(child.ID), tokenA, "", 200, tenantA.ID)
	})
	t.Run("recheck_membership_tenant_and_user_status", func(t *testing.T) {
		membership := tx.Model(&model.Membership{}).Where("tenant_id = ? AND user_id = ?", tenantA.ID, userA.ID)
		checkWorkspaces := func(want ...tenant.Workspace) {
			t.Helper()
			data := call(t, "GET", workspaces, tokenA, "", 200)
			var got []tenant.Workspace
			requireOK(t, json.Unmarshal(data, &got))
			if got == nil || !slices.Equal(got, want) {
				t.Fatalf("workspaces: got %+v, want %+v", got, want)
			}
		}
		a := tenant.Workspace{ID: fmt.Sprint(tenantA.ID), Name: tenantA.Name, DepartmentID: fmt.Sprint(deptA.ID), RoleKey: "tenant_admin"}
		b := tenant.Workspace{ID: fmt.Sprint(tenantB.ID), Name: tenantB.Name, DepartmentID: fmt.Sprint(deptB.ID), RoleKey: "sales"}
		checkWorkspaces(a)
		call(t, "GET", workspaces, tokenA+"invalid", "", 401)
		call(t, "GET", workspaces, tokenA, "", 400, tenantB.ID)
		call(t, "GET", workspaces+"?tenant_id="+b.ID, tokenA, "", 400)
		requireOK(t, membership.Update("status", "disabled").Error)
		checkWorkspaces()
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
		requireOK(t, membership.Update("status", "active").Error)
		requireOK(t, tx.Model(&model.Tenant{}).Where("id = ?", tenantA.ID).Update("status", "disabled").Error)
		checkWorkspaces()
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
		requireOK(t, tx.Model(&model.Tenant{}).Where("id = ?", tenantA.ID).Update("status", "active").Error)
		requireOK(t, tx.Model(&userA).Update("enable", 2).Error)
		checkWorkspaces()
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
		requireOK(t, tx.Model(&userA).Update("enable", 1).Error)
		requireOK(t, tx.Create(&model.Membership{
			TenantID: tenantB.ID, UserID: uint64(userA.ID), DepartmentID: deptB.ID, Status: "active", RoleKey: "sales",
		}).Error)
		checkWorkspaces(a, b)
		const menu = "/api/dealflow/menu"
		if got := string(call(t, "GET", menu, tokenA, "", 200, tenantA.ID)); got != `[{"key":"departments","canRename":true}]` {
			t.Fatalf("unexpected admin menu: %s", got)
		}
		if got := string(call(t, "GET", menu, tokenA, "", 200, tenantB.ID)); got != `[{"key":"departments","canRename":false}]` {
			t.Fatalf("unexpected sales menu: %s", got)
		}
		call(t, "GET", url(deptB.ID), tokenA, "", 200, tenantB.ID)
		call(t, "PATCH", url(deptB.ID), tokenA, `{"name":"wrong role"}`, 403, tenantB.ID)
		requireOK(t, tx.Model(&model.Membership{}).Where("tenant_id = ? AND user_id = ?", tenantB.ID, userA.ID).Update("status", "disabled").Error)
		checkWorkspaces(a)
		call(t, "GET", menu, tokenA, "", 403, tenantB.ID)
		call(t, "GET", url(deptB.ID), tokenA, "", 403, tenantB.ID)
		requireOK(t, tx.Delete(&userA).Error)
		checkWorkspaces()
		call(t, "GET", url(deptA.ID), tokenA, "", 403, tenantA.ID)
	})
	t.Run("platform_and_upstream_customer_boundary", func(t *testing.T) {
		path := fmt.Sprintf("/api/dealflow/platform/tenants/%d", tenantB.ID)
		var got model.Tenant
		requireOK(t, json.Unmarshal(call(t, "GET", path, operatorToken, "", 200), &got))
		if got != tenantB {
			t.Fatalf("unexpected platform metadata: %+v", got)
		}
		call(t, "GET", path, tokenB, "", 403)
		call(t, "GET", path, operatorToken, "", 400, tenantB.ID)
		call(t, "GET", url(operatorDept.ID), operatorToken, "", 403, operatorTenant.ID)
		call(t, "GET", "/api/dealflow/menu", operatorToken, "", 403, operatorTenant.ID)
		call(t, "GET", workspaces, operatorToken, "", 403)

		customer := examplemodel.ExaCustomer{CustomerName: "GVA fixture", SysUserID: userB.ID, SysUserAuthorityID: 704}
		requireOK(t, tx.Omit(clause.Associations).Create(&customer).Error)
		global.GVA_DB = tx.Session(&gorm.Session{NewDB: true})
		defer func() { global.GVA_DB = db }()
		customerPath := fmt.Sprintf("/api/customer/customer?ID=%d", customer.ID)
		if data := call(t, "GET", customerPath, tokenB, "", 200); !strings.Contains(string(data), customer.CustomerName) {
			t.Fatalf("upstream customer lookup returned %s", data)
		}
		call(t, "GET", customerPath, operatorToken, "", 403)
		call(t, "GET", "/api/customer/customerList?page=1&pageSize=10", operatorToken, "", 403)

		requireOK(t, tx.Model(&operator).Update("enable", 2).Error)
		call(t, "GET", path, operatorToken, "", 403)
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
