package tenant

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCRMInput            = errors.New("invalid CRM input")
	ErrCustomerNotFound    = errors.New("customer not found")
	ErrContactNotFound     = errors.New("contact not found")
	ErrCustomerHasContacts = errors.New("customer has contacts")
)

type CustomerInput struct {
	Name     string `json:"name" binding:"required" maxLength:"128"`
	Industry string `json:"industry" maxLength:"64"`
	Remark   string `json:"remark" maxLength:"512"`
}

type ContactInput struct {
	Name  string `json:"name" binding:"required" maxLength:"128"`
	Phone string `json:"phone" maxLength:"32"`
	Email string `json:"email" maxLength:"254"`
	Title string `json:"title" maxLength:"64"`
}

type CustomerSummary struct {
	model.Customer
	OwnerName      string `json:"ownerName"`
	DepartmentName string `json:"departmentName"`
}

type CustomerDetail struct {
	CustomerSummary
	CanUpdate   bool `json:"canUpdate"`
	CanDelete   bool `json:"canDelete"`
	CanTransfer bool `json:"canTransfer"`
}

type CustomerPage struct {
	List     []CustomerSummary `json:"list"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type CustomerSearch struct {
	Page, PageSize       int
	Sort, Order, Keyword string
	Followup             string
}

func cleanCustomer(input CustomerInput) (CustomerInput, error) {
	input.Name, input.Industry, input.Remark = strings.TrimSpace(input.Name), strings.TrimSpace(input.Industry), strings.TrimSpace(input.Remark)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 128 || utf8.RuneCountInString(input.Industry) > 64 || utf8.RuneCountInString(input.Remark) > 512 {
		return input, ErrCRMInput
	}
	return input, nil
}

func cleanContact(input ContactInput) (ContactInput, error) {
	input.Name, input.Phone, input.Email, input.Title = strings.TrimSpace(input.Name), strings.TrimSpace(input.Phone), strings.TrimSpace(input.Email), strings.TrimSpace(input.Title)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 128 || utf8.RuneCountInString(input.Phone) > 32 || len(input.Email) > 254 || utf8.RuneCountInString(input.Title) > 64 {
		return input, ErrCRMInput
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || address.Address != input.Email {
			return input, ErrCRMInput
		}
	}
	return input, nil
}

func (s *Service) customerQuery(ctx context.Context, scope TenantContext, action string) (*gorm.DB, error) {
	visibility, err := s.ResolveVisibility(ctx, scope, "customers", action)
	if err != nil {
		return nil, err
	}
	return visibility.ForRecords(s.db.WithContext(ctx).Model(&model.Customer{}))
}

func (s *Service) customerFind(ctx context.Context, scope TenantContext, id uint64, action string, lock bool) (model.Customer, error) {
	var row model.Customer
	query, err := s.customerQuery(ctx, scope, action)
	if err != nil {
		return row, err
	}
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err = query.Where("crm_customers.id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrCustomerNotFound
	}
	return row, err
}

func customerLabels(query *gorm.DB) *gorm.DB {
	return query.Select("crm_customers.*, COALESCE(NULLIF(sys_users.nick_name,''),sys_users.username) AS owner_name, departments.name AS department_name").
		Joins("JOIN sys_users ON sys_users.id=crm_customers.owner_id").
		Joins("JOIN departments ON departments.tenant_id=crm_customers.tenant_id AND departments.id=crm_customers.department_id")
}

func (s *Service) Customers(ctx context.Context, scope TenantContext, input CustomerSearch) (CustomerPage, error) {
	result := CustomerPage{List: make([]CustomerSummary, 0), Page: input.Page, PageSize: input.PageSize}
	if input.Page < 1 || input.Page > 1000000 || input.PageSize < 1 || input.PageSize > 100 || utf8.RuneCountInString(input.Keyword) > 128 {
		return result, ErrCRMInput
	}
	sorts := map[string]string{"id": "id", "name": "name", "code": "code", "nextFollowupAt": "next_followup_at"}
	column, valid := sorts[input.Sort]
	if !valid {
		return result, ErrCRMInput
	}
	if input.Order != "asc" && input.Order != "desc" {
		return result, ErrCRMInput
	}
	if input.Followup != "" && input.Followup != "none" && input.Followup != "overdue" && input.Followup != "scheduled" {
		return result, ErrCRMInput
	}
	if s.db == nil {
		return result, ErrUnavailable
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query, err := New(tx).customerQuery(ctx, scope, "read")
		if err != nil {
			return err
		}
		if input.Keyword != "" {
			query = query.Where("LOCATE(?,crm_customers.name)>0", input.Keyword)
		}
		switch input.Followup {
		case "none":
			query = query.Where("crm_customers.next_followup_at IS NULL")
		case "overdue":
			query = query.Where("crm_customers.next_followup_at < ?", time.Now().UTC())
		case "scheduled":
			query = query.Where("crm_customers.next_followup_at >= ?", time.Now().UTC())
		}
		if err := query.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
			return err
		}
		query = customerLabels(query).Order(clause.OrderByColumn{Column: clause.Column{Table: "crm_customers", Name: column}, Desc: input.Order == "desc"})
		if input.Sort != "id" {
			query = query.Order(clause.OrderByColumn{Column: clause.Column{Table: "crm_customers", Name: "id"}, Desc: input.Order == "desc"})
		}
		return query.Offset((input.Page - 1) * input.PageSize).Limit(input.PageSize).Find(&result.List).Error
	})
	return result, err
}

func validBatchIDs(ids []uint64) bool {
	if len(ids) == 0 || len(ids) > 100 {
		return false
	}
	for _, id := range ids {
		if id == 0 {
			return false
		}
	}
	return true
}

func (s *Service) CustomerBatch(ctx context.Context, scope TenantContext, ids []uint64) ([]CustomerSummary, error) {
	if !validBatchIDs(ids) {
		return nil, ErrCRMInput
	}
	query, err := s.customerQuery(ctx, scope, "read")
	if err != nil {
		return nil, err
	}
	rows := make([]CustomerSummary, 0)
	err = customerLabels(query).Where("crm_customers.id IN ?", ids).Order("crm_customers.id").Find(&rows).Error
	return rows, err
}

func (s *Service) Customer(ctx context.Context, scope TenantContext, id uint64) (CustomerDetail, error) {
	var result CustomerDetail
	query, err := s.customerQuery(ctx, scope, "read")
	if err != nil {
		return result, err
	}
	err = customerLabels(query).Where("crm_customers.id = ?", id).Take(&result.CustomerSummary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, ErrCustomerNotFound
	}
	if err != nil {
		return result, err
	}
	allowed := func(action string) (bool, error) {
		_, err := s.customerFind(ctx, scope, id, action, false)
		if errors.Is(err, ErrCustomerNotFound) || errors.Is(err, ErrTenantDenied) {
			return false, nil
		}
		return err == nil, err
	}
	result.CanUpdate, err = allowed("update")
	if err != nil {
		return CustomerDetail{}, err
	}
	result.CanDelete, err = allowed("delete")
	if err != nil {
		return CustomerDetail{}, err
	}
	if primaryCustomerManager(scope) {
		result.CanTransfer, err = allowed("transfer")
	}
	return result, err
}

// Lock the tenant before resolving the actor so organization changes cannot
// race a customer/contact write. Different customers can still be edited concurrently.
func (s *Service) customerWrite(ctx context.Context, scope TenantContext, work func(*Service, TenantContext) error) error {
	if scope.membership.ID == 0 {
		return ErrTenantDenied
	}
	if s.db == nil {
		return ErrUnavailable
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var enterprise model.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Take(&enterprise, scope.membership.TenantID).Error; err != nil {
			return err
		}
		svc := New(tx)
		current, err := svc.ResolveTenant(ctx, scope.membership.UserID, scope.membership.TenantID)
		if err != nil {
			return err
		}
		if current.membership.ID != scope.membership.ID {
			return ErrTenantDenied
		}
		return work(svc, current)
	})
}

func (s *Service) CreateCustomer(ctx context.Context, scope TenantContext, input CustomerInput) (model.Customer, error) {
	input, err := cleanCustomer(input)
	if err != nil {
		return model.Customer{}, err
	}
	var saved model.Customer
	err = s.customerWrite(ctx, scope, func(svc *Service, current TenantContext) error {
		if err := current.require("customers", "create"); err != nil {
			return err
		}
		saved = model.Customer{TenantID: current.membership.TenantID, OwnerID: current.membership.UserID, DepartmentID: current.membership.DepartmentID,
			Code: "C-" + uuid.NewString(), Name: input.Name, Industry: input.Industry, Remark: input.Remark}
		if err := svc.db.Create(&saved).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: saved.ID, Action: "customer.create", SubjectID: saved.ID})
	})
	if err != nil {
		return model.Customer{}, err
	}
	return saved, nil
}

func (s *Service) UpdateCustomer(ctx context.Context, scope TenantContext, id uint64, input CustomerInput) error {
	input, err := cleanCustomer(input)
	if err != nil {
		return err
	}
	return s.customerWrite(ctx, scope, func(svc *Service, current TenantContext) error {
		row, err := svc.customerFind(ctx, current, id, "update", true)
		if err != nil {
			return err
		}
		if row.Name == input.Name && row.Industry == input.Industry && row.Remark == input.Remark {
			return nil
		}
		if err := svc.db.Model(&model.Customer{}).Where("tenant_id = ? AND id = ?", row.TenantID, row.ID).
			Updates(map[string]any{"name": input.Name, "industry": input.Industry, "remark": input.Remark}).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: row.ID, Action: "customer.update", SubjectID: row.ID})
	})
}

func (s *Service) DeleteCustomer(ctx context.Context, scope TenantContext, id uint64) error {
	return s.customerWrite(ctx, scope, func(svc *Service, current TenantContext) error {
		row, err := svc.customerFind(ctx, current, id, "delete", true)
		if err != nil {
			return err
		}
		var count int64
		if err := svc.db.Model(&model.Contact{}).Where("tenant_id = ? AND customer_id = ?", row.TenantID, row.ID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrCustomerHasContacts
		}
		if err := svc.db.Model(&model.Followup{}).Where("tenant_id = ? AND customer_id = ?", row.TenantID, row.ID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrCustomerHasHistory
		}
		if err := svc.db.Where("tenant_id = ? AND id = ?", row.TenantID, row.ID).Delete(&model.Customer{}).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: row.ID, Action: "customer.delete", SubjectID: row.ID})
	})
}

func (s *Service) Contacts(ctx context.Context, scope TenantContext, customerID uint64) ([]ContactView, error) {
	parent, err := s.customerFind(ctx, scope, customerID, "read", false)
	if err != nil {
		return nil, err
	}
	rows := make([]model.Contact, 0)
	err = s.db.WithContext(ctx).Where("tenant_id = ? AND customer_id = ?", parent.TenantID, parent.ID).Order("id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return s.contactViews(ctx, scope, rows)
}

func (s *Service) contactQuery(ctx context.Context, scope TenantContext, action string) (*gorm.DB, error) {
	query, err := s.customerQuery(ctx, scope, action)
	if err != nil {
		return nil, err
	}
	return query.Joins("JOIN crm_contacts ON crm_contacts.tenant_id=crm_customers.tenant_id AND crm_contacts.customer_id=crm_customers.id").Select("crm_contacts.*"), nil
}

func (s *Service) Contact(ctx context.Context, scope TenantContext, id uint64) (ContactView, error) {
	query, err := s.contactQuery(ctx, scope, "read")
	if err != nil {
		return ContactView{}, err
	}
	var row model.Contact
	err = query.Where("crm_contacts.id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ContactView{}, ErrContactNotFound
	}
	if err != nil {
		return ContactView{}, err
	}
	views, err := s.contactViews(ctx, scope, []model.Contact{row})
	if err != nil {
		return ContactView{}, err
	}
	return views[0], nil
}

func (s *Service) ContactBatch(ctx context.Context, scope TenantContext, ids []uint64) ([]ContactView, error) {
	if !validBatchIDs(ids) {
		return nil, ErrCRMInput
	}
	query, err := s.contactQuery(ctx, scope, "read")
	if err != nil {
		return nil, err
	}
	rows := make([]model.Contact, 0)
	err = query.Where("crm_contacts.id IN ?", ids).Order("crm_contacts.id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return s.contactViews(ctx, scope, rows)
}

func (s *Service) CreateContact(ctx context.Context, scope TenantContext, customerID uint64, input ContactInput) (model.Contact, error) {
	input, err := cleanContact(input)
	if err != nil {
		return model.Contact{}, err
	}
	var saved model.Contact
	err = s.customerWrite(ctx, scope, func(svc *Service, current TenantContext) error {
		parent, err := svc.customerFind(ctx, current, customerID, "update", true)
		if err != nil {
			return err
		}
		saved = model.Contact{TenantID: parent.TenantID, CustomerID: parent.ID, Name: input.Name, Phone: input.Phone, Email: input.Email, Title: input.Title}
		if err := svc.db.Create(&saved).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: parent.ID, Action: "contact.create", SubjectID: saved.ID})
	})
	if err != nil {
		return model.Contact{}, err
	}
	return saved, nil
}

func (s *Service) writeContact(ctx context.Context, scope TenantContext, id uint64, work func(*Service, TenantContext, model.Contact) error) error {
	return s.customerWrite(ctx, scope, func(svc *Service, current TenantContext) error {
		query, err := svc.contactQuery(ctx, current, "update")
		if err != nil {
			return err
		}
		var row model.Contact
		if err := query.Where("crm_contacts.id = ?", id).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrContactNotFound
			}
			return err
		}
		if _, err := svc.customerFind(ctx, current, row.CustomerID, "update", true); err != nil {
			if errors.Is(err, ErrCustomerNotFound) {
				return ErrContactNotFound
			}
			return err
		}
		if err := svc.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND customer_id = ?", row.TenantID, row.ID, row.CustomerID).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrContactNotFound
			}
			return err
		}
		return work(svc, current, row)
	})
}

func (s *Service) UpdateContact(ctx context.Context, scope TenantContext, id uint64, input ContactInput) error {
	input, err := cleanContact(input)
	if err != nil {
		return err
	}
	return s.writeContact(ctx, scope, id, func(svc *Service, current TenantContext, row model.Contact) error {
		if row.Name == input.Name && row.Phone == input.Phone && row.Email == input.Email && row.Title == input.Title {
			return nil
		}
		if err := svc.db.Model(&model.Contact{}).Where("tenant_id = ? AND id = ?", row.TenantID, row.ID).
			Updates(map[string]any{"name": input.Name, "phone": input.Phone, "email": input.Email, "title": input.Title}).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: row.CustomerID, Action: "contact.update", SubjectID: row.ID})
	})
}

func (s *Service) DeleteContact(ctx context.Context, scope TenantContext, id uint64) error {
	return s.writeContact(ctx, scope, id, func(svc *Service, current TenantContext, row model.Contact) error {
		if err := svc.db.Where("tenant_id = ? AND id = ?", row.TenantID, row.ID).Delete(&model.Contact{}).Error; err != nil {
			return err
		}
		return svc.writeAudit(ctx, current, model.AuditEvent{CustomerID: row.CustomerID, Action: "contact.delete", SubjectID: row.ID})
	})
}
