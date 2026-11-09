package tenant

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	active            = "active"
	defaultDepartment = "general"
	administrator     = "tenant_admin"
)

var tenantCode = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service { return &Service{db: db} }

type ProvisionInput struct {
	Code        string
	Name        string
	AdminUserID uint64
}

// Provision binds an existing GVA account as the tenant's first administrator.
func (s *Service) Provision(ctx context.Context, input ProvisionInput) (model.Tenant, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !tenantCode.MatchString(input.Code) || input.Name == "" || utf8.RuneCountInString(input.Name) > 128 || input.AdminUserID == 0 {
		return model.Tenant{}, errors.New("provide a valid tenant code, name and administrator user ID")
	}

	var created model.Tenant
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := checkUser(tx, input.AdminUserID); err != nil {
			return err
		}
		var err error
		created, err = createTenant(tx, input, []member{{input.AdminUserID, administrator}})
		return err
	})
	if err != nil {
		return model.Tenant{}, err
	}
	return created, nil
}

type member struct {
	userID uint64
	role   string
}

func checkUser(tx *gorm.DB, id uint64) error {
	var user system.SysUser
	err := tx.Select("id").Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ? AND enable = ?", id, 1).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("GVA user %d is missing, disabled or deleted", id)
	}
	if err != nil {
		return fmt.Errorf("verify GVA user %d: %w", id, err)
	}
	return nil
}

func createTenant(tx *gorm.DB, input ProvisionInput, members []member) (model.Tenant, error) {
	tenant := model.Tenant{Code: input.Code, Name: input.Name, Status: active}
	if err := tx.Create(&tenant).Error; err != nil {
		return model.Tenant{}, fmt.Errorf("create tenant: %w", err)
	}
	department := model.Department{TenantID: tenant.ID, Code: defaultDepartment, Name: "综合部"}
	if err := tx.Create(&department).Error; err != nil {
		return model.Tenant{}, fmt.Errorf("create default department: %w", err)
	}
	for _, m := range members {
		binding := model.Membership{
			TenantID: tenant.ID, DepartmentID: department.ID,
			UserID: m.userID, RoleKey: m.role, Status: active,
		}
		if err := tx.Create(&binding).Error; err != nil {
			return model.Tenant{}, fmt.Errorf("create membership: %w", err)
		}
	}
	return tenant, nil
}
