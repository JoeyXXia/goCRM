package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tenantapi "github.com/flipped-aurora/gin-vue-admin/server/api/v1/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	tenantrouter "github.com/flipped-aurora/gin-vue-admin/server/router/tenant"
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

// Opt-in local server for the browser test. Normal CI skips this fixture.
func TestDay10BrowserFixture(t *testing.T) {
	fixturePath := os.Getenv("DEALFLOW_BROWSER_FIXTURE_FILE")
	if fixturePath == "" {
		t.Skip("browser fixture is opt-in")
	}
	cfg, err := testmysql.Config(os.Getenv("DEALFLOW_TEST_DSN"))
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
	ctx := context.Background()
	requireOK(t, migrate(ctx, pool, true))
	users := []system.SysUser{
		{Username: "d10-admin-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1, NickName: "测试管理员"},
		{Username: "d10-sales-a-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1, NickName: "销售甲"},
		{Username: "d10-sales-b-" + uuid.NewString(), UUID: uuid.New(), Password: "!", Enable: 1, NickName: "销售乙"},
	}
	requireOK(t, db.Omit(clause.Associations).Create(&users).Error)
	t.Cleanup(func() {
		requireOK(t, db.Unscoped().Where("id IN ?", []uint{users[0].ID, users[1].ID, users[2].ID}).Delete(&system.SysUser{}).Error)
	})
	ids := make([]uint64, 0, 2)
	t.Cleanup(func() {
		if len(ids) == 0 {
			return
		}
		requireOK(t, db.Where("tenant_id IN ?", ids).Delete(&model.Contact{}).Error)
		requireOK(t, db.Where("tenant_id IN ?", ids).Delete(&model.Followup{}).Error)
		requireOK(t, db.Where("tenant_id IN ?", ids).Delete(&model.AuditEvent{}).Error)
		requireOK(t, db.Where("tenant_id IN ?", ids).Delete(&model.Customer{}).Error)
		requireOK(t, db.Where("tenant_id IN ?", ids).Delete(&model.Membership{}).Error)
		requireOK(t, db.Model(&model.Department{}).Where("tenant_id IN ?", ids).Update("parent_id", nil).Error)
		requireOK(t, db.Where("tenant_id IN ?", ids).Delete(&model.Department{}).Error)
		requireOK(t, db.Where("id IN ?", ids).Delete(&model.Tenant{}).Error)
	})
	svc := tenant.New(db)
	a, err := svc.Provision(ctx, tenant.ProvisionInput{Code: "d10-a-" + uuid.NewString(), Name: "Day10 企业 A", AdminUserID: uint64(users[0].ID)})
	requireOK(t, err)
	ids = append(ids, a.ID)
	b, err := svc.Provision(ctx, tenant.ProvisionInput{Code: "d10-b-" + uuid.NewString(), Name: "Day10 企业 B", AdminUserID: uint64(users[0].ID)})
	requireOK(t, err)
	ids = append(ids, b.ID)
	for _, entry := range []struct {
		enterprise uint64
		user       uint
		role       string
	}{{a.ID, users[1].ID, "sales"}, {b.ID, users[1].ID, "finance"}, {b.ID, users[2].ID, "sales"}} {
		scope, err := svc.ResolveTenant(ctx, uint64(users[0].ID), entry.enterprise)
		requireOK(t, err)
		var department model.Department
		requireOK(t, db.Where("tenant_id = ?", entry.enterprise).Take(&department).Error)
		_, err = svc.SaveMember(ctx, scope, 0, tenant.MemberInput{UserID: uint64(entry.user), DepartmentID: department.ID, RoleKey: entry.role, Status: "active"})
		requireOK(t, err)
	}
	scopeB, err := svc.ResolveTenant(ctx, uint64(users[2].ID), b.ID)
	requireOK(t, err)
	if os.Getenv("DEALFLOW_BROWSER_SCENARIO") == "day11" {
		scope, err := svc.ResolveTenant(ctx, uint64(users[0].ID), a.ID)
		requireOK(t, err)
		department, err := svc.SaveDepartment(ctx, scope, 0, tenant.DepartmentInput{Code: "d11-target", Name: "转移目标团队"})
		requireOK(t, err)
		_, err = svc.SaveMember(ctx, scope, 0, tenant.MemberInput{UserID: uint64(users[2].ID), DepartmentID: department.ID, RoleKey: "sales", Status: "active"})
		requireOK(t, err)
	}
	foreign, err := svc.CreateCustomer(ctx, scopeB, tenant.CustomerInput{Name: "企业 B 客户", Industry: "物流"})
	requireOK(t, err)
	_, err = svc.CreateContact(ctx, scopeB, foreign.ID, tenant.ContactInput{Name: "B 联系人", Email: "b@example.test"})
	requireOK(t, err)
	oldDB, oldConfig, oldCache, oldLog := global.GVA_DB, global.GVA_CONFIG, global.BlackCache, global.GVA_LOG
	t.Cleanup(func() {
		global.GVA_DB, global.GVA_CONFIG, global.BlackCache, global.GVA_LOG = oldDB, oldConfig, oldCache, oldLog
	})
	global.GVA_DB = db
	global.GVA_LOG = zap.NewNop()
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: uuid.NewString(), ExpiresTime: "1h", BufferTime: "1m"}
	global.GVA_CONFIG.System.RouterPrefix = ""
	global.BlackCache = local_cache.NewCache(local_cache.SetDefaultExpire(time.Hour))
	jwt := utils.NewJWT()
	token, err := jwt.CreateToken(jwt.CreateClaims(request.BaseClaims{ID: users[1].ID, UUID: users[1].UUID, AuthorityId: 704}))
	requireOK(t, err)
	adminToken, err := jwt.CreateToken(jwt.CreateClaims(request.BaseClaims{ID: users[0].ID, UUID: users[0].UUID, AuthorityId: 704}))
	requireOK(t, err)
	newOwnerToken, err := jwt.CreateToken(jwt.CreateClaims(request.BaseClaims{ID: users[2].ID, UUID: users[2].UUID, AuthorityId: 704}))
	requireOK(t, err)
	gin.SetMode(gin.TestMode)
	mux := gin.New()
	tenantrouter.Register(mux.Group(""), tenantapi.PlatformAPI{Operator: uuid.New(), DB: func() *gorm.DB { return db }})
	mux.GET("/base/captcha", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"captchaId": "fixture", "picPath": ""}, "msg": "ok"})
	})
	mux.POST("/sysError/createSysError", func(c *gin.Context) { c.JSON(200, gin.H{"code": 0, "data": nil, "msg": "ok"}) })
	stop := make(chan struct{})
	var once sync.Once
	key := uuid.NewString()
	mux.POST("/__day10/stop", func(c *gin.Context) {
		if c.Query("key") != key {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusOK)
		once.Do(func() { close(stop) })
	})
	listener, err := net.Listen("tcp", "127.0.0.1:8888")
	requireOK(t, err)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() { server.Close() })
	data, err := json.Marshal(map[string]any{"token": token, "adminToken": adminToken, "newOwnerToken": newOwnerToken, "targetUserID": users[2].ID, "tenantA": a.ID, "tenantB": b.ID, "foreignCustomer": foreign.ID, "stopKey": key})
	requireOK(t, err)
	requireOK(t, os.MkdirAll(filepath.Dir(fixturePath), 0700))
	requireOK(t, os.WriteFile(fixturePath, data, 0600))
	t.Cleanup(func() { os.Remove(fixturePath) })
	t.Log("Day 10 browser fixture ready on 127.0.0.1:8888")
	select {
	case <-stop:
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Minute):
		t.Error("browser fixture timed out")
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	requireOK(t, server.Shutdown(shutdown))
}
