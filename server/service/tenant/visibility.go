package tenant

import (
	"context"
	"strconv"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Visibility contains only server-resolved values and is valid for one request.
type Visibility struct {
	membership    model.Membership
	ranges        []string
	departmentIDs []uint64
}

func (v Visibility) PermissionVersion() uint64 { return v.membership.PermissionVersion }

func (s *Service) ResolveVisibility(ctx context.Context, scope TenantContext, resource, action string) (Visibility, error) {
	if scope.membership.ID == 0 {
		return Visibility{}, ErrTenantDenied
	}
	current, err := s.ResolveTenant(ctx, scope.membership.UserID, scope.membership.TenantID)
	if err != nil {
		return Visibility{}, err
	}
	if current.membership.ID != scope.membership.ID || current.membership.PermissionVersion == 0 {
		return Visibility{}, ErrTenantDenied
	}
	policy, err := enterprisePolicy()
	if err != nil {
		return Visibility{}, err
	}
	domain := strconv.FormatUint(current.membership.TenantID, 10)
	roles := append([]string{current.membership.RoleKey}, current.membership.DataRoles...)
	ranges := map[string]bool{}
	for _, role := range roles {
		allowed, err := policy.Enforce(enterpriseMember{Domain: domain, Role: role}, domain, resource, action)
		if err != nil {
			return Visibility{}, err
		}
		if !allowed {
			continue
		}
		rules, err := policy.GetFilteredPolicy(0, role, resource, action)
		if err != nil {
			return Visibility{}, err
		}
		for _, rule := range rules {
			switch rule[3] {
			case "SELF", "DEPARTMENT", "DEPARTMENT_AND_CHILDREN", "ALL":
				ranges[rule[3]] = true
			default:
				return Visibility{}, ErrTenantDenied
			}
		}
	}
	if len(ranges) == 0 {
		return Visibility{}, ErrTenantDenied
	}
	result := Visibility{membership: current.membership}
	if ranges["ALL"] {
		result.ranges = []string{"ALL"}
		return result, nil
	}
	for _, key := range []string{"SELF", "DEPARTMENT", "DEPARTMENT_AND_CHILDREN", "ALL"} {
		if ranges[key] {
			result.ranges = append(result.ranges, key)
		}
	}
	if ranges["DEPARTMENT_AND_CHILDREN"] {
		result.departmentIDs, err = s.descendantIDs(ctx, current.membership.TenantID, current.membership.DepartmentID)
		if err != nil {
			return Visibility{}, err
		}
	}
	return result, nil
}

func (s *Service) descendantIDs(ctx context.Context, tenantID, root uint64) ([]uint64, error) {
	var rows []model.Department
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	children := map[uint64][]uint64{}
	exists := false
	for _, row := range rows {
		if row.ID == root {
			exists = true
		}
		if row.ParentID != nil {
			children[*row.ParentID] = append(children[*row.ParentID], row.ID)
		}
	}
	if !exists {
		return nil, ErrTenantDenied
	}
	ids := []uint64{root}
	seen := map[uint64]bool{root: true}
	for i := 0; i < len(ids); i++ {
		for _, id := range children[ids[i]] {
			if seen[id] {
				return nil, ErrOrganizationConflict
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ForRecords filters a model with tenant_id, owner_id and department_id columns.
// The caller supplies its model/query, while all authorization values stay private.
func (v Visibility) ForRecords(query *gorm.DB) (*gorm.DB, error) {
	return v.filter(query, "owner_id")
}

func (v Visibility) filter(query *gorm.DB, ownerColumn string) (*gorm.DB, error) {
	m := v.membership
	if m.ID == 0 || m.UserID == 0 || m.TenantID == 0 || m.PermissionVersion == 0 || len(v.ranges) == 0 {
		return nil, ErrTenantDenied
	}
	if query == nil {
		return nil, ErrUnavailable
	}
	column := func(name string) clause.Column { return clause.Column{Table: clause.CurrentTable, Name: name} }
	grants := make([]clause.Expression, 0, len(v.ranges))
	for _, dataRange := range v.ranges {
		switch dataRange {
		case "SELF":
			grants = append(grants, clause.Eq{Column: column(ownerColumn), Value: m.UserID})
		case "DEPARTMENT":
			grants = append(grants, clause.Eq{Column: column("department_id"), Value: m.DepartmentID})
		case "DEPARTMENT_AND_CHILDREN":
			if len(v.departmentIDs) == 0 {
				return nil, ErrTenantDenied
			}
			values := make([]any, len(v.departmentIDs))
			for i, id := range v.departmentIDs {
				values[i] = id
			}
			grants = append(grants, clause.IN{Column: column("department_id"), Values: values})
		case "ALL":
			grants = append(grants, clause.Expr{SQL: "1=1"})
		default:
			return nil, ErrTenantDenied
		}
	}
	allowed := grants[0]
	if len(grants) > 1 {
		allowed = clause.Or(grants...)
	}
	authorization := clause.And(clause.Eq{Column: column("tenant_id"), Value: m.TenantID}, allowed)
	// Apply at execution time so existing and subsequently chained OR filters
	// are grouped together before intersecting them with the authorization.
	return query.Scopes(func(tx *gorm.DB) *gorm.DB {
		var condition clause.Expression = authorization
		if previous := tx.Statement.Clauses["WHERE"].Expression; previous != nil {
			where, ok := previous.(clause.Where)
			if !ok {
				tx.AddError(ErrTenantDenied)
				return tx
			}
			condition = clause.Expr{
				SQL:  "(?) AND (?)",
				Vars: []any{clause.AndConditions{Exprs: where.Exprs}, authorization},
			}
		}
		tx.Statement.Clauses["WHERE"] = clause.Clause{
			Name:       "WHERE",
			Expression: clause.Where{Exprs: []clause.Expression{condition}},
		}
		return tx
	}), nil
}
