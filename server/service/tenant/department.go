package tenant

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"gorm.io/gorm"
)

var (
	ErrDepartmentNotFound = errors.New("department not found")
	ErrInvalidName        = errors.New("invalid department name")
)

func (s *Service) Department(ctx context.Context, scope TenantContext, id uint64) (model.Department, error) {
	if err := scope.require("departments", "read"); err != nil {
		return model.Department{}, err
	}
	return departmentRepository{s.db}.find(ctx, scope, id)
}

func (s *Service) RenameDepartment(ctx context.Context, scope TenantContext, id uint64, name string) error {
	if err := scope.require("departments", "rename"); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 128 {
		return ErrInvalidName
	}
	return departmentRepository{s.db}.rename(ctx, scope, id, name)
}

type departmentRepository struct{ db *gorm.DB }

func (r departmentRepository) scoped(ctx context.Context, scope TenantContext, id uint64) (*gorm.DB, error) {
	if scope.membership.ID == 0 || scope.membership.TenantID == 0 || scope.membership.UserID == 0 {
		return nil, ErrTenantDenied
	}
	if r.db == nil {
		return nil, ErrUnavailable
	}
	return r.db.WithContext(ctx).Model(&model.Department{}).
		Where("tenant_id = ? AND id = ?", scope.membership.TenantID, id), nil
}

func (r departmentRepository) find(ctx context.Context, scope TenantContext, id uint64) (model.Department, error) {
	var row model.Department
	query, err := r.scoped(ctx, scope, id)
	if err != nil {
		return row, err
	}
	err = query.Select("id", "tenant_id", "code", "name").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrDepartmentNotFound
	}
	return row, err
}

func (r departmentRepository) rename(ctx context.Context, scope TenantContext, id uint64, name string) error {
	query, err := r.scoped(ctx, scope, id)
	if err != nil {
		return err
	}
	result := query.Update("name", name)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}
	// MySQL reports zero changed rows for a no-op rename.
	_, err = r.find(ctx, scope, id)
	return err
}
