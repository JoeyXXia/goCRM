package tenant

import (
	"context"
	"errors"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"gorm.io/gorm"
)

var (
	ErrTenantDenied = errors.New("tenant access denied")
	ErrUnavailable  = errors.New("tenant database unavailable")
)

// TenantContext is valid for the request in which ResolveTenant checked membership.
type TenantContext struct {
	membership model.Membership
}

func (s *Service) ResolveTenant(ctx context.Context, userID, tenantID uint64) (TenantContext, error) {
	if userID == 0 || tenantID == 0 {
		return TenantContext{}, ErrTenantDenied
	}
	if s.db == nil {
		return TenantContext{}, ErrUnavailable
	}
	var membership model.Membership
	err := s.activeMemberships(ctx, userID).
		Select("tenant_memberships.*").
		Where("tenant_memberships.tenant_id = ?", tenantID).
		Take(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return TenantContext{}, ErrTenantDenied
	}
	if err != nil {
		return TenantContext{}, err
	}
	return TenantContext{membership: membership}, nil
}

type Workspace struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DepartmentID string `json:"departmentId"`
	RoleKey      string `json:"roleKey"`
}

func (s *Service) Workspaces(ctx context.Context, userID uint64) ([]Workspace, error) {
	if userID == 0 {
		return nil, ErrTenantDenied
	}
	if s.db == nil {
		return nil, ErrUnavailable
	}
	items := make([]Workspace, 0)
	err := s.activeMemberships(ctx, userID).
		Select("CAST(tenants.id AS CHAR) AS id, tenants.name, CAST(tenant_memberships.department_id AS CHAR) AS department_id, tenant_memberships.role_key").
		Order("tenants.id").
		Scan(&items).Error
	return items, err
}

func (s *Service) activeMemberships(ctx context.Context, userID uint64) *gorm.DB {
	return s.db.WithContext(ctx).Model(&model.Membership{}).
		Joins("JOIN tenants ON tenants.id = tenant_memberships.tenant_id").
		Joins("JOIN sys_users ON sys_users.id = tenant_memberships.user_id").
		Where("tenant_memberships.user_id = ?", userID).
		Where("tenant_memberships.status = ? AND tenants.status = ?", active, active).
		Where("sys_users.enable = ? AND sys_users.deleted_at IS NULL", 1)
}
