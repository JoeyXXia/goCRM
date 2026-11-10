package tenant

import (
	"context"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrTenantNotFound = errors.New("tenant not found")

// PlatformTenant returns tenant metadata after revalidating the configured operator.
func (s *Service) PlatformTenant(ctx context.Context, operator uuid.UUID, userID uint, id uint64) (model.Tenant, error) {
	var row model.Tenant
	if operator == uuid.Nil || userID == 0 {
		return row, ErrTenantDenied
	}
	if s.db == nil {
		return row, ErrUnavailable
	}
	var user system.SysUser
	err := s.db.WithContext(ctx).Select("id").Where("id = ? AND uuid = ? AND enable = 1", userID, operator).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrTenantDenied
	}
	if err != nil {
		return row, err
	}
	err = s.db.WithContext(ctx).Select("id", "code", "name", "status").Take(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrTenantNotFound
	}
	return row, err
}
