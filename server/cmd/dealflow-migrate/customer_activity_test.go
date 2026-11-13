package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type activityFixture struct {
	db                   *gorm.DB
	ctx                  context.Context
	enterprise, foreign  model.Tenant
	department           model.Department
	admin, oldOwner      system.SysUser
	adminToken, oldToken string
	call                 func(*testing.T, string, string, string, string, int, ...uint64) json.RawMessage
}

func verifyCustomerActivity(t *testing.T, f activityFixture) {
	svc := tenant.New(f.db)
	scope, err := svc.ResolveTenant(f.ctx, uint64(f.admin.ID), f.enterprise.ID)
	requireOK(t, err)
	source, err := svc.SaveDepartment(f.ctx, scope, 0, tenant.DepartmentInput{Code: "d11-source", Name: "原团队", ParentID: &f.department.ID})
	requireOK(t, err)
	target, err := svc.SaveDepartment(f.ctx, scope, 0, tenant.DepartmentInput{Code: "d11-target", Name: "接收团队", ParentID: &f.department.ID})
	requireOK(t, err)
	_, err = svc.SaveMember(f.ctx, scope, 0, tenant.MemberInput{UserID: uint64(f.oldOwner.ID), DepartmentID: source.ID, RoleKey: "sales", Status: "active"})
	requireOK(t, err)
	newMember := func(name, role string, dept uint64) (system.SysUser, string) {
		user := system.SysUser{Username: "d11-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1, NickName: name}
		requireOK(t, f.db.Omit(clause.Associations).Create(&user).Error)
		_, err := svc.SaveMember(f.ctx, scope, 0, tenant.MemberInput{UserID: uint64(user.ID), DepartmentID: dept, RoleKey: role, Status: "active"})
		requireOK(t, err)
		jwt := utils.NewJWT()
		token, err := jwt.CreateToken(jwt.CreateClaims(request.BaseClaims{ID: user.ID, UUID: user.UUID, AuthorityId: 704}))
		requireOK(t, err)
		return user, token
	}
	recipient, newToken := newMember("接收销售", "sales", target.ID)
	_, managerToken := newMember("原团队经理", "manager", source.ID)
	_, readerToken := newMember("只读财务", "finance", source.ID)
	call := func(method, path, token, body string, status int) json.RawMessage {
		return f.call(t, method, path, token, body, status, f.enterprise.ID)
	}
	var customer model.Customer
	requireOK(t, json.Unmarshal(call("POST", "/api/dealflow/customers", f.oldToken, `{"name":"转移客户","remark":"private@example.test"}`, 200), &customer))
	base := fmt.Sprintf("/api/dealflow/customers/%d", customer.ID)
	var contact model.Contact
	requireOK(t, json.Unmarshal(call("POST", base+"/contacts", f.oldToken, `{"name":"私密联系人","phone":"13800138000","email":"secret@example.test"}`, 200), &contact))
	contactURL := fmt.Sprintf("/api/dealflow/contacts/%d", contact.ID)
	var masked tenant.ContactView
	requireOK(t, json.Unmarshal(call("GET", contactURL, readerToken, "", 200), &masked))
	if !masked.Masked || masked.Phone != "138****8000" || masked.Email != "s***@example.test" {
		t.Fatalf("contact masking failed: %+v", masked)
	}
	for _, path := range []string{base + "/contacts", fmt.Sprintf("/api/dealflow/contacts/batch?ids=%d", masked.ID)} {
		var rows []tenant.ContactView
		requireOK(t, json.Unmarshal(call("GET", path, readerToken, "", 200), &rows))
		if len(rows) != 1 || !rows[0].Masked || rows[0].Phone != masked.Phone || rows[0].Email != masked.Email {
			t.Fatal("list or batch bypassed contact masking")
		}
	}
	call("POST", base+"/followups", readerToken, `{"content":"no write"}`, 403)
	call("POST", base+"/transfer", f.oldToken, fmt.Sprintf(`{"ownerId":"%d"}`, recipient.ID), 403)
	call("POST", base+"/transfer", managerToken, fmt.Sprintf(`{"ownerId":"%d"}`, recipient.ID), 400)
	call("POST", base+"/transfer", f.adminToken, fmt.Sprintf(`{"ownerId":"%d"}`, f.admin.ID+999999), 400)
	call("POST", base+"/followups", f.oldToken, `{"content":"bad","customerId":"1"}`, 400)
	call("POST", base+"/followups", f.oldToken, `{"content":"bad","nextFollowupAt":"invalid"}`, 400)
	call("POST", base+"/followups", f.oldToken, `{"content":"bad","nextFollowupAt":"0999-01-01T00:00:00Z"}`, 400)
	foreignScope, err := svc.ResolveTenant(f.ctx, uint64(f.oldOwner.ID), f.foreign.ID)
	requireOK(t, err)
	foreign, err := svc.CreateCustomer(f.ctx, foreignScope, tenant.CustomerInput{Name: "异企业客户"})
	requireOK(t, err)
	call("POST", fmt.Sprintf("/api/dealflow/customers/%d/followups", foreign.ID), f.oldToken, `{"content":"foreign"}`, 404)
	next := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	body := fmt.Sprintf(`{"content":"私密跟进 13800138000 secret@example.test","nextFollowupAt":"%s"}`, next.Format(time.RFC3339Nano))
	// The failure happens after the business writes, before their audit insert.
	func() {
		hook := "test:reject_day11_audit"
		injected := errors.New("injected audit failure")
		requireOK(t, f.db.Callback().Create().Before("gorm:create").Register(hook, func(tx *gorm.DB) {
			if tx.Statement.Table == "crm_audit_events" {
				tx.AddError(injected)
			}
		}))
		defer func() { requireOK(t, f.db.Callback().Create().Remove(hook)) }()
		call("POST", base+"/followups", f.oldToken, body, 500)
		call("POST", base+"/transfer", f.adminToken, fmt.Sprintf(`{"ownerId":"%d"}`, recipient.ID), 500)
	}()
	var count int64
	requireOK(t, f.db.Model(&model.Followup{}).Where("customer_id = ?", customer.ID).Count(&count).Error)
	if count != 0 {
		t.Fatal("failed audit left followup rows")
	}
	var unchanged model.Customer
	requireOK(t, f.db.Take(&unchanged, customer.ID).Error)
	if unchanged.OwnerID != uint64(f.oldOwner.ID) || unchanged.DepartmentID != source.ID || unchanged.NextFollowupAt != nil {
		t.Fatal("failed audit left business changes")
	}
	past := time.Now().UTC().Add(-24 * time.Hour)
	call("POST", base+"/followups", f.oldToken, fmt.Sprintf(`{"content":"昨日跟进","nextFollowupAt":"%s"}`, past.Format(time.RFC3339)), 200)
	var page tenant.CustomerPage
	requireOK(t, json.Unmarshal(call("GET", "/api/dealflow/customers?followup=overdue", f.oldToken, "", 200), &page))
	if page.Total != 1 || page.List[0].ID != customer.ID {
		t.Fatal("overdue filter did not match")
	}
	call("POST", base+"/followups", f.oldToken, body, 200)
	var history tenant.FollowupPage
	requireOK(t, json.Unmarshal(call("GET", base+"/followups?page=1&pageSize=1", f.oldToken, "", 200), &history))
	if history.Total != 2 || len(history.List) != 1 || !strings.Contains(history.List[0].Content, "私密跟进") {
		t.Fatal("timeline ordering/pagination failed")
	}
	requireOK(t, json.Unmarshal(call("GET", "/api/dealflow/customers?followup=scheduled&sort=nextFollowupAt&order=asc", f.oldToken, "", 200), &page))
	if page.Total != 1 {
		t.Fatal("scheduled filter failed")
	}
	call("GET", "/api/dealflow/customers?followup=unknown", f.oldToken, "", 400)
	requireOK(t, f.db.Model(&system.SysUser{}).Where("id = ?", recipient.ID).Update("enable", 2).Error)
	call("POST", base+"/transfer", f.adminToken, fmt.Sprintf(`{"ownerId":"%d"}`, recipient.ID), 400)
	requireOK(t, f.db.Model(&system.SysUser{}).Where("id = ?", recipient.ID).Update("enable", 1).Error)
	var targets []tenant.TransferTarget
	requireOK(t, json.Unmarshal(call("GET", base+"/transfer-targets", f.adminToken, "", 200), &targets))
	found := false
	for _, item := range targets {
		if item.UserID == fmt.Sprint(recipient.ID) {
			found = true
		}
	}
	if !found {
		t.Fatal("eligible recipient missing")
	}
	transfer := fmt.Sprintf(`{"ownerId":"%d"}`, recipient.ID)
	call("POST", base+"/transfer", f.adminToken, transfer, 200)
	call("POST", base+"/transfer", f.adminToken, transfer, 200)
	for _, path := range []string{base, base + "/contacts", base + "/followups", contactURL} {
		call("GET", path, f.oldToken, "", 404)
	}
	call("PUT", contactURL, f.oldToken, `{"name":"old owner"}`, 404)
	call("POST", base+"/followups", f.oldToken, `{"content":"old owner"}`, 404)
	var invisible []tenant.CustomerSummary
	requireOK(t, json.Unmarshal(call("GET", "/api/dealflow/customers/batch?ids="+fmt.Sprint(customer.ID), f.oldToken, "", 200), &invisible))
	if len(invisible) != 0 {
		t.Fatal("old owner still sees batch records")
	}
	var detail tenant.CustomerDetail
	requireOK(t, json.Unmarshal(call("GET", base, newToken, "", 200), &detail))
	if detail.OwnerID != uint64(recipient.ID) || detail.DepartmentID != target.ID || detail.NextFollowupAt == nil || !detail.CanUpdate {
		t.Fatal("transfer did not update ownership and department")
	}
	requireOK(t, json.Unmarshal(call("GET", contactURL, newToken, "", 200), &masked))
	if masked.Masked || masked.Phone != "13800138000" {
		t.Fatal("new owner cannot access contact")
	}
	call("GET", base+"/followups", newToken, "", 200)
	call("GET", base+"/audit", newToken, "", 403)
	var events []tenant.AuditView
	requireOK(t, json.Unmarshal(call("GET", base+"/audit", f.adminToken, "", 200), &events))
	transfers := 0
	for _, event := range events {
		if _, err := uuid.Parse(event.RequestID); err != nil {
			t.Fatal("audit request ID missing")
		}
		if event.Action == "customer.transfer" {
			transfers++
			if event.FromOwnerID != uint64(f.oldOwner.ID) || event.ToOwnerID != uint64(recipient.ID) || event.FromDepartmentID != source.ID || event.ToDepartmentID != target.ID {
				t.Fatal("transfer audit identifiers wrong")
			}
		}
	}
	if transfers != 1 {
		t.Fatalf("idempotent transfer wrote %d events", transfers)
	}
	encoded, err := json.Marshal(events)
	requireOK(t, err)
	for _, secret := range []string{"13800138000", "secret@example.test", "private@example.test", "私密跟进", "私密联系人"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("audit leaked private data")
		}
	}
	call("DELETE", contactURL, newToken, "", 200)
	call("DELETE", base, f.adminToken, "", 409)
	call("POST", base+"/followups", newToken, `{"content":"取消下次安排","nextFollowupAt":null}`, 200)
	requireOK(t, json.Unmarshal(call("GET", base, newToken, "", 200), &detail))
	if detail.NextFollowupAt != nil {
		t.Fatal("explicit null did not clear next followup")
	}
}
