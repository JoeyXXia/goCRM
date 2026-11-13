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
	t.Run("day10_customers_and_contacts", func(t *testing.T) {
		base := "/api/dealflow"
		adminScope, err := svc.ResolveTenant(ctx, uint64(userA.ID), tenantA.ID)
		requireOK(t, err)
		member, err := svc.SaveMember(ctx, adminScope, 0, tenant.MemberInput{UserID: uint64(userB.ID), DepartmentID: deptA.ID, RoleKey: "sales", Status: "active"})
		requireOK(t, err)
		create := func(token, name string, enterprise uint64) model.Customer {
			var row model.Customer
			requireOK(t, json.Unmarshal(call(t, "POST", base+"/customers", token, fmt.Sprintf(`{"name":"%s","industry":"设备","remark":"初次联系"}`, name), 200, enterprise), &row))
			return row
		}
		own := create(tokenB, "Zulu", tenantA.ID)
		peer := create(tokenA, "Alpha", tenantA.ID)
		foreign := create(tokenB, "Beta", tenantB.ID)
		if own.OwnerID != uint64(userB.ID) || own.TenantID != tenantA.ID || own.DepartmentID != deptA.ID || len(own.Code) != 38 || own.Code == peer.Code {
			t.Fatal("server assignments or business code invalid")
		}
		customerURL := func(id uint64) string { return fmt.Sprintf("%s/customers/%d", base, id) }
		contactURL := func(id uint64) string { return fmt.Sprintf("%s/contacts/%d", base, id) }
		for _, field := range []string{"ownerId", "tenantId", "departmentId", "code"} {
			body := fmt.Sprintf(`{"name":"forged","%s":"%d"}`, field, tenantB.ID)
			call(t, "POST", base+"/customers", tokenB, body, 400, tenantA.ID)
			call(t, "PUT", customerURL(own.ID), tokenB, body, 400, tenantA.ID)
		}
		for _, id := range []uint64{peer.ID, foreign.ID} {
			call(t, "GET", customerURL(id), tokenB, "", 404, tenantA.ID)
			call(t, "PUT", customerURL(id), tokenB, `{"name":"tampered"}`, 404, tenantA.ID)
			call(t, "DELETE", customerURL(id), tokenB, "", 404, tenantA.ID)
		}
		encoded, err := json.Marshal(tenant.CustomerInput{Name: strings.Repeat("<", 128), Industry: strings.Repeat("<", 64), Remark: strings.Repeat("<", 512)})
		requireOK(t, err)
		var maximum model.Customer
		requireOK(t, json.Unmarshal(call(t, "POST", base+"/customers", tokenB, string(encoded), 200, tenantA.ID), &maximum))
		call(t, "DELETE", customerURL(maximum.ID), tokenB, "", 200, tenantA.ID)
		call(t, "GET", customerURL(foreign.ID), tokenA, "", 403, tenantB.ID)
		var page tenant.CustomerPage
		requireOK(t, json.Unmarshal(call(t, "GET", base+"/customers?page=1&pageSize=1&sort=name&order=asc", tokenA, "", 200, tenantA.ID), &page))
		if page.Total != 2 || len(page.List) != 1 || page.List[0].ID != peer.ID || page.List[0].OwnerName == "" || page.List[0].DepartmentName == "" {
			t.Fatalf("unexpected customer page: %+v", page)
		}
		requireOK(t, json.Unmarshal(call(t, "GET", base+"/customers?keyword=Alpha", tokenB, "", 200, tenantA.ID), &page))
		if page.Total != 0 || len(page.List) != 0 {
			t.Fatal("search leaked another owner's customer")
		}
		for _, query := range []string{"sort=owner_id", "sort=name%20DESC", "page=0", "pageSize=101", "page=1&page=2", "tenantId=1", "scope=ALL", "ids=1"} {
			call(t, "GET", base+"/customers?"+query, tokenB, "", 400, tenantA.ID)
		}
		var batch []tenant.CustomerSummary
		ids := fmt.Sprintf("%d,%d,%d", peer.ID, own.ID, foreign.ID)
		requireOK(t, json.Unmarshal(call(t, "GET", base+"/customers/batch?ids="+ids, tokenB, "", 200, tenantA.ID), &batch))
		if len(batch) != 1 || batch[0].ID != own.ID {
			t.Fatal("customer batch escaped visibility")
		}
		call(t, "GET", base+"/customers/batch?ids=0", tokenB, "", 400, tenantA.ID)
		call(t, "PUT", customerURL(own.ID), tokenB, `{"name":"Zulu edited","industry":"","remark":""}`, 200, tenantA.ID)
		var detail tenant.CustomerDetail
		requireOK(t, json.Unmarshal(call(t, "GET", customerURL(own.ID), tokenB, "", 200, tenantA.ID), &detail))
		if detail.Name != "Zulu edited" || detail.OwnerID != own.OwnerID || detail.Code != own.Code || !detail.CanUpdate || !detail.CanDelete {
			t.Fatal("customer update changed ownership or controls")
		}
		addContact := func(customer uint64, token string, enterprise uint64) model.Contact {
			var row model.Contact
			requireOK(t, json.Unmarshal(call(t, "POST", customerURL(customer)+"/contacts", token, `{"name":"联系人","phone":"12345","email":"contact@example.test","title":"采购"}`, 200, enterprise), &row))
			return row
		}
		person := addContact(own.ID, tokenB, tenantA.ID)
		peerPerson := addContact(peer.ID, tokenA, tenantA.ID)
		foreignPerson := addContact(foreign.ID, tokenB, tenantB.ID)
		for _, id := range []uint64{peerPerson.ID, foreignPerson.ID} {
			call(t, "GET", contactURL(id), tokenB, "", 404, tenantA.ID)
			call(t, "PUT", contactURL(id), tokenB, `{"name":"tampered"}`, 404, tenantA.ID)
			call(t, "DELETE", contactURL(id), tokenB, "", 404, tenantA.ID)
		}
		call(t, "POST", customerURL(foreign.ID)+"/contacts", tokenB, `{"name":"foreign"}`, 404, tenantA.ID)
		call(t, "GET", customerURL(peer.ID)+"/contacts", tokenB, "", 404, tenantA.ID)
		call(t, "POST", customerURL(own.ID)+"/contacts", tokenB, `{"name":"bad","customerId":"1"}`, 400, tenantA.ID)
		call(t, "PUT", contactURL(person.ID), tokenB, `{"name":"bad","customerId":"1"}`, 400, tenantA.ID)
		call(t, "PUT", contactURL(person.ID), tokenB, `{"name":"bad","email":"invalid"}`, 400, tenantA.ID)
		call(t, "PUT", contactURL(person.ID), tokenB, `{"name":"联系人更新","phone":"","email":"","title":""}`, 200, tenantA.ID)
		var contacts []model.Contact
		ids = fmt.Sprintf("%d,%d,%d", peerPerson.ID, person.ID, foreignPerson.ID)
		requireOK(t, json.Unmarshal(call(t, "GET", base+"/contacts/batch?ids="+ids, tokenB, "", 200, tenantA.ID), &contacts))
		if len(contacts) != 1 || contacts[0].ID != person.ID || contacts[0].Name != "联系人更新" || contacts[0].Email != "" {
			t.Fatal("contact batch or update invalid")
		}
		_, err = svc.SaveMember(ctx, adminScope, member.ID, tenant.MemberInput{UserID: member.UserID, DepartmentID: deptA.ID, RoleKey: "sales", Status: "active", DataRoles: []string{"finance"}})
		requireOK(t, err)
		requireOK(t, json.Unmarshal(call(t, "GET", customerURL(peer.ID), tokenB, "", 200, tenantA.ID), &detail))
		if detail.CanUpdate || detail.CanDelete {
			t.Fatal("read grant incorrectly granted write controls")
		}
		call(t, "GET", contactURL(peerPerson.ID), tokenB, "", 200, tenantA.ID)
		call(t, "PUT", contactURL(peerPerson.ID), tokenB, `{"name":"read only"}`, 404, tenantA.ID)
		call(t, "DELETE", customerURL(own.ID), tokenB, "", 409, tenantA.ID)
		badCustomer := model.Customer{TenantID: tenantA.ID, Code: "C-" + uuid.NewString(), Name: "bad", OwnerID: uint64(operator.ID), DepartmentID: deptA.ID}
		expectForeignKeyViolation(t, tx.Create(&badCustomer).Error)
		badCustomer.ID = 0
		badCustomer.Code = "C-" + uuid.NewString()
		badCustomer.OwnerID = uint64(userA.ID)
		badCustomer.DepartmentID = deptB.ID
		expectForeignKeyViolation(t, tx.Create(&badCustomer).Error)
		expectForeignKeyViolation(t, tx.Create(&model.Contact{TenantID: tenantA.ID, CustomerID: foreign.ID, Name: "bad"}).Error)
		call(t, "DELETE", contactURL(person.ID), tokenB, "", 200, tenantA.ID)
		call(t, "DELETE", customerURL(own.ID), tokenB, "", 200, tenantA.ID)
		call(t, "GET", contactURL(person.ID), tokenB, "", 404, tenantA.ID)
		requireOK(t, svc.DeleteMember(ctx, adminScope, member.ID))
		call(t, "DELETE", contactURL(peerPerson.ID), tokenA, "", 200, tenantA.ID)
		call(t, "DELETE", customerURL(peer.ID), tokenA, "", 200, tenantA.ID)
		call(t, "DELETE", contactURL(foreignPerson.ID), tokenB, "", 200, tenantB.ID)
		call(t, "DELETE", customerURL(foreign.ID), tokenB, "", 200, tenantB.ID)
	})
	t.Run("day9_visibility_matrix_and_permission_refresh", func(t *testing.T) {
		adminScope, err := svc.ResolveTenant(ctx, uint64(userA.ID), tenantA.ID)
		requireOK(t, err)
		team, err := svc.SaveDepartment(ctx, adminScope, 0, tenant.DepartmentInput{Code: "d9-team", Name: "Team", ParentID: &deptA.ID})
		requireOK(t, err)
		child, err := svc.SaveDepartment(ctx, adminScope, 0, tenant.DepartmentInput{Code: "d9-child", Name: "Child", ParentID: &team.ID})
		requireOK(t, err)
		sibling, err := svc.SaveDepartment(ctx, adminScope, 0, tenant.DepartmentInput{Code: "d9-sibling", Name: "Sibling", ParentID: &deptA.ID})
		requireOK(t, err)
		actor, err := svc.SaveMember(ctx, adminScope, 0, tenant.MemberInput{UserID: uint64(userB.ID), DepartmentID: team.ID, RoleKey: "sales", Status: "active"})
		requireOK(t, err)
		makeMember := func(department uint64) model.Membership {
			user := system.SysUser{Username: "d9-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1}
			requireOK(t, tx.Omit(clause.Associations).Create(&user).Error)
			row, err := svc.SaveMember(ctx, adminScope, 0, tenant.MemberInput{UserID: uint64(user.ID), DepartmentID: department, RoleKey: "sales", Status: "active"})
			requireOK(t, err)
			return row
		}
		peer, lower, other := makeMember(team.ID), makeMember(child.ID), makeMember(sibling.ID)
		var admin model.Membership
		requireOK(t, tx.Where("tenant_id = ? AND user_id = ?", tenantA.ID, userA.ID).Take(&admin).Error)
		staleScope, err := svc.ResolveTenant(ctx, uint64(userB.ID), tenantA.ID)
		requireOK(t, err)
		// Temporary records exercise the reusable owner/department filter before Day 10.
		requireOK(t, tx.Exec("CREATE TEMPORARY TABLE day9_records (id BIGINT UNSIGNED PRIMARY KEY, tenant_id BIGINT UNSIGNED NOT NULL, owner_id BIGINT UNSIGNED NOT NULL, department_id BIGINT UNSIGNED NOT NULL) ENGINE=InnoDB").Error)
		defer func() { requireOK(t, tx.Exec("DROP TEMPORARY TABLE day9_records").Error) }()
		for _, row := range []struct{ id, tenantID, owner, department uint64 }{
			{1, tenantA.ID, uint64(userB.ID), team.ID},
			{2, tenantA.ID, peer.UserID, team.ID},
			{3, tenantA.ID, lower.UserID, child.ID},
			{4, tenantA.ID, other.UserID, sibling.ID},
			{5, tenantA.ID, uint64(userB.ID), sibling.ID},
			{6, tenantB.ID, uint64(userB.ID), deptB.ID},
			{7, tenantB.ID, peer.UserID, team.ID},
		} {
			requireOK(t, tx.Exec("INSERT INTO day9_records VALUES (?,?,?,?)", row.id, row.tenantID, row.owner, row.department).Error)
		}
		if _, err := (tenant.Visibility{}).ForRecords(tx.Table("day9_records")); !errors.Is(err, tenant.ErrTenantDenied) {
			t.Fatal("zero visibility must deny")
		}
		if _, err := svc.ResolveVisibility(ctx, staleScope, "contracts", "read"); !errors.Is(err, tenant.ErrTenantDenied) {
			t.Fatal("unmapped resource must deny")
		}
		if _, err := svc.ResolveVisibility(ctx, staleScope, "members", "write"); !errors.Is(err, tenant.ErrTenantDenied) {
			t.Fatal("unmapped action must deny")
		}
		for _, tc := range []struct {
			name, role       string
			extra            []string
			records, members []uint64
		}{
			{"self", "sales", nil, []uint64{1, 5}, []uint64{actor.ID}},
			{"department", "finance", nil, []uint64{1, 2}, []uint64{actor.ID, peer.ID}},
			{"descendants", "manager", nil, []uint64{1, 2, 3}, []uint64{actor.ID, peer.ID, lower.ID}},
			{"all", "tenant_admin", nil, []uint64{1, 2, 3, 4, 5}, []uint64{admin.ID, actor.ID, peer.ID, lower.ID, other.ID}},
			{"all_with_secondary_tree", "tenant_admin", []string{"manager"}, []uint64{1, 2, 3, 4, 5}, []uint64{admin.ID, actor.ID, peer.ID, lower.ID, other.ID}},
			{"role_union", "sales", []string{"finance"}, []uint64{1, 2, 5}, []uint64{actor.ID, peer.ID}},
			{"three_role_union", "sales", []string{"manager", "finance"}, []uint64{1, 2, 3, 5}, []uint64{actor.ID, peer.ID, lower.ID}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				body, err := json.Marshal(tenant.MemberInput{UserID: actor.UserID, DepartmentID: team.ID, RoleKey: tc.role, Status: "active", DataRoles: tc.extra})
				requireOK(t, err)
				var updated model.Membership
				data := call(t, "PUT", fmt.Sprintf("/api/dealflow/members/%d", actor.ID), tokenA, string(body), 200, tenantA.ID)
				requireOK(t, json.Unmarshal(data, &updated))
				visibility, err := svc.ResolveVisibility(ctx, staleScope, "members", "read")
				requireOK(t, err)
				if visibility.PermissionVersion() != updated.PermissionVersion {
					t.Fatal("stale membership version reused")
				}
				query, err := visibility.ForRecords(tx.Table("day9_records").Where("id = ?", 6).Or("id <> ?", 6))
				requireOK(t, err)
				var ids []uint64
				requireOK(t, query.Or("1=1").Order("id").Pluck("id", &ids).Error)
				if !slices.Equal(ids, tc.records) {
					t.Fatalf("records %v, want %v", ids, tc.records)
				}
				query, err = visibility.ForRecords(tx.Table("day9_records").Where("id = ?\nOR id <> ?", 6, 6))
				requireOK(t, err)
				ids = nil
				requireOK(t, query.Order("id").Pluck("id", &ids).Error)
				if !slices.Equal(ids, tc.records) {
					t.Fatalf("multiline OR records %v, want %v", ids, tc.records)
				}
				var members []model.Membership
				data = call(t, "GET", "/api/dealflow/members", tokenB, `{"scope":"ALL"}`, 200, tenantA.ID)
				requireOK(t, json.Unmarshal(data, &members))
				ids = nil
				for _, row := range members {
					ids = append(ids, row.ID)
				}
				slices.Sort(tc.members)
				if !slices.Equal(ids, tc.members) {
					t.Fatalf("members %v, want %v", ids, tc.members)
				}
			})
		}
		call(t, "GET", "/api/dealflow/members?scope=ALL", tokenB, "", 400, tenantA.ID)
		_, err = svc.SaveMember(ctx, adminScope, actor.ID, tenant.MemberInput{UserID: actor.UserID, DepartmentID: team.ID, RoleKey: "sales", Status: "active", DataRoles: []string{"tenant_admin"}})
		if !errors.Is(err, tenant.ErrOrganizationInput) {
			t.Fatal("data roles must not grant organization administration")
		}
		input := tenant.MemberInput{UserID: actor.UserID, DepartmentID: team.ID, RoleKey: "sales", Status: "active", DataRoles: []string{"sales", "finance", "finance"}}
		updated, err := svc.SaveMember(ctx, adminScope, actor.ID, input)
		requireOK(t, err)
		input.DataRoles = []string{"finance", "sales"}
		again, err := svc.SaveMember(ctx, adminScope, actor.ID, input)
		requireOK(t, err)
		if again.PermissionVersion != updated.PermissionVersion {
			t.Fatal("equivalent role sets changed version")
		}
		input.DataRoles = nil
		input.Status = "disabled"
		_, err = svc.SaveMember(ctx, adminScope, actor.ID, input)
		requireOK(t, err)
		if _, err := svc.ResolveVisibility(ctx, staleScope, "members", "read"); !errors.Is(err, tenant.ErrTenantDenied) {
			t.Fatal("disabled stale scope accepted")
		}
		call(t, "GET", "/api/dealflow/members", tokenB, "", 403, tenantA.ID)
		requireOK(t, svc.DeleteMember(ctx, adminScope, actor.ID))
		for _, row := range []model.Membership{peer, lower, other} {
			requireOK(t, svc.DeleteMember(ctx, adminScope, row.ID))
		}
		requireOK(t, svc.DeleteDepartment(ctx, adminScope, child.ID))
		requireOK(t, svc.DeleteDepartment(ctx, adminScope, team.ID))
		requireOK(t, svc.DeleteDepartment(ctx, adminScope, sibling.ID))
	})
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
		requireOK(t, tx.Model(&system.SysUser{}).Where("id = ?", userB.ID).Update("enable", 2).Error)
		call(t, "DELETE", adminURL, tokenA, "", 409, tenantA.ID)
		requireOK(t, tx.Model(&system.SysUser{}).Where("id = ?", userB.ID).Update("enable", 1).Error)
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
	t.Run("day11_customer_activity", func(t *testing.T) {
		verifyCustomerActivity(t, activityFixture{db: tx, ctx: ctx, enterprise: tenantA, foreign: tenantB, department: deptA,
			admin: userA, oldOwner: userB, adminToken: tokenA, oldToken: tokenB, call: call})
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
		if got := string(call(t, "GET", menu, tokenA, "", 200, tenantA.ID)); got != `[{"key":"departments","canRename":true},{"key":"customers","canRename":false,"canCreate":true}]` {
			t.Fatalf("unexpected admin menu: %s", got)
		}
		if got := string(call(t, "GET", menu, tokenA, "", 200, tenantB.ID)); got != `[{"key":"departments","canRename":false},{"key":"customers","canRename":false,"canCreate":true}]` {
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
