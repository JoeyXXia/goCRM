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
	err := s.db.WithContext(ctx).Model(&model.Membership{}).
		Select("tenant_memberships.*").
		Joins("JOIN tenants ON tenants.id = tenant_memberships.tenant_id").
		Joins("JOIN sys_users ON sys_users.id = tenant_memberships.user_id").
		Where("tenant_memberships.user_id = ? AND tenant_memberships.tenant_id = ?", userID, tenantID).
		Where("tenant_memberships.status = ? AND tenants.status = ?", active, active).
		Where("sys_users.enable = ? AND sys_users.deleted_at IS NULL", 1).
		Take(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return TenantContext{}, ErrTenantDenied
	}
	if err != nil {
		return TenantContext{}, err
	}
	return TenantContext{membership: membership}, nil
}
