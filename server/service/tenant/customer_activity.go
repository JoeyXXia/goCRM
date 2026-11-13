package tenant

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrCustomerHasHistory = errors.New("customer has followup history")

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	if _, err := uuid.Parse(id); err != nil {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, id)
}

func (s *Service) writeAudit(ctx context.Context, scope TenantContext, event model.AuditEvent) error {
	event.TenantID = scope.membership.TenantID
	event.ActorID = scope.membership.UserID
	event.RequestID, _ = ctx.Value(requestIDKey{}).(string)
	event.CreatedAt = time.Now().UTC().Truncate(time.Millisecond)
	return s.db.WithContext(ctx).Create(&event).Error
}

type FollowupInput struct {
	Content        string     `json:"content" binding:"required" maxLength:"1024"`
	NextFollowupAt *time.Time `json:"nextFollowupAt"`
}

type FollowupView struct {
	model.Followup
	ActorName string `json:"actorName"`
}

type FollowupPage struct {
	List     []FollowupView `json:"list"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

type TransferInput struct {
	OwnerID uint64 `json:"ownerId,string" swaggertype:"string" binding:"required"`
}

type TransferTarget struct {
	UserID         string `json:"userId"`
	Name           string `json:"name"`
	DepartmentID   string `json:"departmentId"`
	DepartmentName string `json:"departmentName"`
}

type AuditView struct {
	model.AuditEvent
	ActorName string `json:"actorName"`
}

func (s *Service) AddFollowup(ctx context.Context, scope TenantContext, customerID uint64, input FollowupInput) (model.Followup, error) {
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" || utf8.RuneCountInString(input.Content) > 1024 {
		return model.Followup{}, ErrCRMInput
	}
	if input.NextFollowupAt != nil {
		next := input.NextFollowupAt.UTC().Truncate(time.Millisecond)
		if next.Year() < 1000 || next.Year() > 9999 {
			return model.Followup{}, ErrCRMInput
		}
		input.NextFollowupAt = &next
	}
	var saved model.Followup
	err := s.customerWrite(ctx, scope, func(svc *Service, current TenantContext) error {
		parent, err := svc.customerFind(ctx, current, customerID, "update", true)
		if err != nil {
			return err
		}
		saved = model.Followup{TenantID: parent.TenantID, CustomerID: parent.ID, ActorID: current.membership.UserID, Content: input.Content, CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
		if err := svc.db.Create(&saved).Error; err != nil {
			return err
		}
		if err := svc.db.Model(&model.Customer{}).Where("tenant_id = ? AND id = ?", parent.TenantID, parent.ID).Update("next_followup_at", input.NextFollowupAt).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: parent.ID, Action: "followup.create", SubjectID: saved.ID})
	})
	if err != nil {
		return model.Followup{}, err
	}
	return saved, nil
}

func (s *Service) Followups(ctx context.Context, scope TenantContext, customerID uint64, page, size int) (FollowupPage, error) {
	result := FollowupPage{List: make([]FollowupView, 0), Page: page, PageSize: size}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return result, ErrCRMInput
	}
	if s.db == nil {
		return result, ErrUnavailable
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		parent, err := New(tx).customerFind(ctx, scope, customerID, "read", false)
		if err != nil {
			return err
		}
		query := tx.Model(&model.Followup{}).Where("crm_followups.tenant_id = ? AND crm_followups.customer_id = ?", parent.TenantID, parent.ID)
		if err := query.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
			return err
		}
		return query.Select("crm_followups.*, COALESCE(NULLIF(sys_users.nick_name,''),sys_users.username,'已移除成员') AS actor_name").
			Joins("LEFT JOIN sys_users ON sys_users.id=crm_followups.actor_id").
			Order("crm_followups.created_at DESC, crm_followups.id DESC").Offset((page - 1) * size).Limit(size).Find(&result.List).Error
	})
	return result, err
}

func primaryCustomerManager(scope TenantContext) bool {
	return scope.membership.RoleKey == administrator || scope.membership.RoleKey == "manager"
}

func (s *Service) transferMembers(ctx context.Context, scope TenantContext) (*gorm.DB, error) {
	visibility, err := s.ResolveVisibility(ctx, scope, "customers", "transfer")
	if err != nil {
		return nil, err
	}
	// The target set is all active memberships in the manager's allowed departments.
	query := s.db.WithContext(ctx).Model(&model.Membership{}).
		Joins("JOIN sys_users ON sys_users.id=tenant_memberships.user_id").
		Where("tenant_memberships.status = ? AND sys_users.enable=1 AND sys_users.deleted_at IS NULL", active).
		Where("tenant_memberships.role_key IN ?", []string{administrator, "sales", "manager"})
	return visibility.filter(query, "user_id")
}

func (s *Service) TransferTargets(ctx context.Context, scope TenantContext, customerID uint64) ([]TransferTarget, error) {
	current, err := s.ResolveTenant(ctx, scope.membership.UserID, scope.membership.TenantID)
	if err != nil {
		return nil, err
	}
	if current.membership.ID != scope.membership.ID || !primaryCustomerManager(current) {
		return nil, ErrTenantDenied
	}
	parent, err := s.customerFind(ctx, current, customerID, "transfer", false)
	if err != nil {
		return nil, err
	}
	query, err := s.transferMembers(ctx, current)
	if err != nil {
		return nil, err
	}
	rows := make([]TransferTarget, 0)
	err = query.Joins("JOIN departments ON departments.tenant_id=tenant_memberships.tenant_id AND departments.id=tenant_memberships.department_id").
		Where("tenant_memberships.user_id <> ?", parent.OwnerID).
		Select("CAST(tenant_memberships.user_id AS CHAR) AS user_id, COALESCE(NULLIF(sys_users.nick_name,''),sys_users.username) AS name, CAST(departments.id AS CHAR) AS department_id, departments.name AS department_name").
		Order("tenant_memberships.user_id").Scan(&rows).Error
	return rows, err
}

func (s *Service) TransferCustomer(ctx context.Context, scope TenantContext, customerID uint64, input TransferInput) error {
	if input.OwnerID == 0 {
		return ErrCRMInput
	}
	return s.customerWrite(ctx, scope, func(svc *Service, current TenantContext) error {
		if !primaryCustomerManager(current) {
			return ErrTenantDenied
		}
		parent, err := svc.customerFind(ctx, current, customerID, "transfer", true)
		if err != nil {
			return err
		}
		query, err := svc.transferMembers(ctx, current)
		if err != nil {
			return err
		}
		var target model.Membership
		err = query.Select("tenant_memberships.*").Where("tenant_memberships.user_id = ?", input.OwnerID).Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCRMInput
		}
		if err != nil {
			return err
		}
		if err := checkUser(svc.db, target.UserID); err != nil {
			return err
		}
		if parent.OwnerID == target.UserID && parent.DepartmentID == target.DepartmentID {
			return nil
		}
		if err := svc.db.Model(&model.Customer{}).Where("tenant_id = ? AND id = ?", parent.TenantID, parent.ID).
			Updates(map[string]any{"owner_id": target.UserID, "department_id": target.DepartmentID}).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: parent.ID, Action: "customer.transfer", SubjectID: parent.ID,
			FromOwnerID: parent.OwnerID, ToOwnerID: target.UserID, FromDepartmentID: parent.DepartmentID, ToDepartmentID: target.DepartmentID})
	})
}

func (s *Service) CustomerAudit(ctx context.Context, scope TenantContext, customerID uint64) ([]AuditView, error) {
	current, err := s.ResolveTenant(ctx, scope.membership.UserID, scope.membership.TenantID)
	if err != nil {
		return nil, err
	}
	if current.membership.ID != scope.membership.ID || !primaryCustomerManager(current) {
		return nil, ErrTenantDenied
	}
	parent, err := s.customerFind(ctx, current, customerID, "transfer", false)
	if err != nil {
		return nil, err
	}
	rows := make([]AuditView, 0)
	err = s.db.WithContext(ctx).Model(&model.AuditEvent{}).
		Select("crm_audit_events.*, COALESCE(NULLIF(sys_users.nick_name,''),sys_users.username,'已移除成员') AS actor_name").
		Joins("LEFT JOIN sys_users ON sys_users.id=crm_audit_events.actor_id").
		Where("tenant_id = ? AND customer_id = ?", parent.TenantID, parent.ID).
		Order("crm_audit_events.created_at DESC, crm_audit_events.id DESC").Limit(100).Find(&rows).Error
	return rows, err
}

type ContactView struct {
	model.Contact
	Masked bool `json:"masked"`
}

func (s *Service) canUpdateCustomer(ctx context.Context, scope TenantContext, id uint64) (bool, error) {
	_, err := s.customerFind(ctx, scope, id, "update", false)
	if errors.Is(err, ErrCustomerNotFound) || errors.Is(err, ErrTenantDenied) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) contactViews(ctx context.Context, scope TenantContext, rows []model.Contact) ([]ContactView, error) {
	result := make([]ContactView, 0, len(rows))
	permissions := map[uint64]bool{}
	for _, row := range rows {
		canEdit, known := permissions[row.CustomerID]
		if !known {
			var err error
			canEdit, err = s.canUpdateCustomer(ctx, scope, row.CustomerID)
			if err != nil {
				return nil, err
			}
			permissions[row.CustomerID] = canEdit
		}
		if !canEdit {
			phone := []rune(row.Phone)
			if len(phone) > 7 {
				row.Phone = string(phone[:3]) + strings.Repeat("*", len(phone)-7) + string(phone[len(phone)-4:])
			} else {
				row.Phone = strings.Repeat("*", len(phone))
			}
			if at := strings.LastIndexByte(row.Email, '@'); at > 0 {
				local := []rune(row.Email[:at])
				row.Email = string(local[0]) + "***" + row.Email[at:]
			}
		}
		result = append(result, ContactView{Contact: row, Masked: !canEdit})
	}
	return result, nil
}
