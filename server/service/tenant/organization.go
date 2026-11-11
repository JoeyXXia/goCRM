package tenant

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrOrganizationInput    = errors.New("invalid organization input")
	ErrOrganizationConflict = errors.New("organization constraint conflict")
	ErrMemberNotFound       = errors.New("member not found")
)

type DepartmentInput struct {
	Code     string  `json:"code" binding:"required" maxLength:"64"`
	Name     string  `json:"name" binding:"required" maxLength:"128"`
	ParentID *uint64 `json:"parentId,string,omitempty" swaggertype:"string"`
}

type MemberInput struct {
	UserID       uint64 `json:"userId,string" swaggertype:"string" binding:"required"`
	DepartmentID uint64 `json:"departmentId,string" swaggertype:"string" binding:"required"`
	RoleKey      string `json:"roleKey" binding:"required" enums:"tenant_admin,sales,manager,finance"`
	Status       string `json:"status" binding:"required" enums:"active,disabled"`
}

// organizationWrite serializes tree moves and administrator changes per tenant.
// It revalidates the actor after acquiring the lock, including a waiting request.
func (s *Service) organizationWrite(ctx context.Context, scope TenantContext, work func(*gorm.DB, uint64) error) error {
	if err := scope.require("organization", "manage"); err != nil {
		return err
	}
	if s.db == nil {
		return ErrUnavailable
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var enterprise model.Tenant
		id := scope.membership.TenantID
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&enterprise, id).Error; err != nil {
			return err
		}
		current, err := New(tx).ResolveTenant(ctx, scope.membership.UserID, id)
		if err != nil {
			return err
		}
		if err := current.require("organization", "manage"); err != nil {
			return err
		}
		return work(tx, id)
	})
}

func (s *Service) Departments(ctx context.Context, scope TenantContext) ([]model.Department, error) {
	if err := scope.require("departments", "read"); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, ErrUnavailable
	}
	rows := make([]model.Department, 0)
	err := s.db.WithContext(ctx).Where("tenant_id = ?", scope.membership.TenantID).Order("id").Find(&rows).Error
	return rows, err
}

func departmentIn(tx *gorm.DB, tenantID, id uint64) (model.Department, error) {
	var row model.Department
	err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrDepartmentNotFound
	}
	return row, err
}

func ancestors(tx *gorm.DB, tenantID, id, forbidden uint64) ([]model.Department, error) {
	rows := make([]model.Department, 0)
	seen := map[uint64]bool{}
	for id != 0 {
		if id == forbidden || seen[id] {
			return nil, ErrOrganizationConflict
		}
		seen[id] = true
		row, err := departmentIn(tx, tenantID, id)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		if row.ParentID == nil {
			break
		}
		id = *row.ParentID
	}
	return rows, nil
}

// Ancestors returns the immediate parent first and the root last.
func (s *Service) Ancestors(ctx context.Context, scope TenantContext, id uint64) ([]model.Department, error) {
	if err := scope.require("departments", "read"); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, ErrUnavailable
	}
	// Keep the chain consistent with a concurrent tree move.
	var rows []model.Department
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var enterprise model.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Take(&enterprise, scope.membership.TenantID).Error; err != nil {
			return err
		}
		row, err := departmentIn(tx, enterprise.ID, id)
		if err != nil {
			return err
		}
		rows = make([]model.Department, 0)
		if row.ParentID == nil {
			return nil
		}
		rows, err = ancestors(tx, enterprise.ID, *row.ParentID, row.ID)
		return err
	})
	return rows, err
}

// SaveDepartment creates a department when id is zero, otherwise replaces its fields.
func (s *Service) SaveDepartment(ctx context.Context, scope TenantContext, id uint64, input DepartmentInput) (model.Department, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !tenantCode.MatchString(input.Code) || input.Name == "" || utf8.RuneCountInString(input.Name) > 128 ||
		(input.ParentID != nil && *input.ParentID == 0) {
		return model.Department{}, ErrOrganizationInput
	}
	var saved model.Department
	err := s.organizationWrite(ctx, scope, func(tx *gorm.DB, tenantID uint64) error {
		if id != 0 {
			if _, err := departmentIn(tx, tenantID, id); err != nil {
				return err
			}
		}
		if input.ParentID != nil {
			if _, err := ancestors(tx, tenantID, *input.ParentID, id); err != nil {
				return err
			}
		}
		saved = model.Department{ID: id, TenantID: tenantID, Code: input.Code, Name: input.Name, ParentID: input.ParentID}
		if id == 0 {
			return tx.Create(&saved).Error
		}
		return tx.Model(&model.Department{}).Where("tenant_id = ? AND id = ?", tenantID, id).
			Updates(map[string]any{"code": input.Code, "name": input.Name, "parent_id": input.ParentID}).Error
	})
	if err != nil {
		return model.Department{}, err
	}
	return saved, nil
}

func (s *Service) DeleteDepartment(ctx context.Context, scope TenantContext, id uint64) error {
	return s.organizationWrite(ctx, scope, func(tx *gorm.DB, tenantID uint64) error {
		if _, err := departmentIn(tx, tenantID, id); err != nil {
			return err
		}
		var children, members int64
		if err := tx.Model(&model.Department{}).Where("tenant_id = ? AND parent_id = ?", tenantID, id).Count(&children).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Membership{}).Where("tenant_id = ? AND department_id = ?", tenantID, id).Count(&members).Error; err != nil {
			return err
		}
		if children != 0 || members != 0 {
			return ErrOrganizationConflict
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.Department{}).Error
	})
}

func (s *Service) Members(ctx context.Context, scope TenantContext) ([]model.Membership, error) {
	if err := scope.require("organization", "manage"); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, ErrUnavailable
	}
	rows := make([]model.Membership, 0)
	err := s.db.WithContext(ctx).Where("tenant_id = ?", scope.membership.TenantID).Order("id").Find(&rows).Error
	return rows, err
}

func lastAdministrator(tx *gorm.DB, tenantID uint64, old model.Membership, remainsAdmin bool) error {
	if old.RoleKey != administrator || old.Status != active || remainsAdmin {
		return nil
	}
	var count int64
	err := tx.Model(&model.Membership{}).Where("tenant_id = ? AND role_key = ? AND status = ?", tenantID, administrator, active).Count(&count).Error
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrOrganizationConflict
	}
	return nil
}

func (s *Service) SaveMember(ctx context.Context, scope TenantContext, id uint64, input MemberInput) (model.Membership, error) {
	if input.UserID == 0 || input.DepartmentID == 0 || (input.Status != active && input.Status != "disabled") {
		return model.Membership{}, ErrOrganizationInput
	}
	switch input.RoleKey {
	case administrator, "sales", "manager", "finance":
	default:
		return model.Membership{}, ErrOrganizationInput
	}
	var saved model.Membership
	err := s.organizationWrite(ctx, scope, func(tx *gorm.DB, tenantID uint64) error {
		if _, err := departmentIn(tx, tenantID, input.DepartmentID); err != nil {
			return err
		}
		if id == 0 || input.Status == active {
			if err := checkUser(tx, input.UserID); err != nil {
				return err
			}
		}
		if id == 0 {
			saved = model.Membership{TenantID: tenantID, UserID: input.UserID, DepartmentID: input.DepartmentID,
				RoleKey: input.RoleKey, Status: input.Status, PermissionVersion: 1}
			return tx.Create(&saved).Error
		}
		err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).Take(&saved).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemberNotFound
		}
		if err != nil {
			return err
		}
		if saved.UserID != input.UserID {
			return ErrOrganizationInput
		}
		if err := lastAdministrator(tx, tenantID, saved, input.RoleKey == administrator && input.Status == active); err != nil {
			return err
		}
		if saved.DepartmentID == input.DepartmentID && saved.RoleKey == input.RoleKey && saved.Status == input.Status {
			return nil
		}
		if saved.PermissionVersion == ^uint64(0) {
			return ErrOrganizationConflict
		}
		err = tx.Model(&model.Membership{}).Where("tenant_id = ? AND id = ?", tenantID, id).Updates(map[string]any{
			"department_id": input.DepartmentID, "role_key": input.RoleKey, "status": input.Status,
			"permission_version": gorm.Expr("permission_version + 1"),
		}).Error
		if err != nil {
			return err
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, id).Take(&saved).Error
	})
	if err != nil {
		return model.Membership{}, err
	}
	return saved, nil
}

func (s *Service) DeleteMember(ctx context.Context, scope TenantContext, id uint64) error {
	return s.organizationWrite(ctx, scope, func(tx *gorm.DB, tenantID uint64) error {
		var row model.Membership
		err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemberNotFound
		}
		if err != nil {
			return err
		}
		if err := lastAdministrator(tx, tenantID, row, false); err != nil {
			return err
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.Membership{}).Error
	})
}
