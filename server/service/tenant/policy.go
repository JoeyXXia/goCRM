package tenant

import (
	"strconv"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

// The membership row is the role binding. The model checks its domain against
// the target domain and applies the DealFlow role matrix independently of GVA.
const enterpriseModel = `
[request_definition]
r = member, dom, obj, act
[policy_definition]
p = role, obj, act, scope
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.member.Domain == r.dom && r.member.Role == p.role && r.obj == p.obj && r.act == p.act
`

type enterpriseMember struct {
	Domain string
	Role   string
}

var enterprisePolicy = sync.OnceValues(func() (*casbin.SyncedEnforcer, error) {
	m, err := model.NewModelFromString(enterpriseModel)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, err
	}
	var rules [][]string
	for _, role := range []string{administrator, "sales", "manager", "finance"} {
		rules = append(rules, []string{role, "departments", "read", ""}, []string{role, "menu", "view", ""})
	}
	rules = append(rules, []string{administrator, "departments", "rename", ""})
	rules = append(rules, []string{administrator, "organization", "manage", ""})
	rules = append(rules,
		[]string{administrator, "members", "read", "ALL"},
		[]string{"sales", "members", "read", "SELF"},
		[]string{"finance", "members", "read", "DEPARTMENT"},
		[]string{"manager", "members", "read", "DEPARTMENT_AND_CHILDREN"},
	)
	_, err = enforcer.AddPolicies(rules)
	return enforcer, err
})

func (scope TenantContext) allows(domain uint64, resource, action string) (bool, error) {
	membership := scope.membership
	if membership.ID == 0 || membership.TenantID == 0 || membership.UserID == 0 {
		return false, nil
	}
	enforcer, err := enterprisePolicy()
	if err != nil {
		return false, err
	}
	member := enterpriseMember{Domain: strconv.FormatUint(membership.TenantID, 10), Role: membership.RoleKey}
	return enforcer.Enforce(member, strconv.FormatUint(domain, 10), resource, action)
}

func (scope TenantContext) require(resource, action string) error {
	allowed, err := scope.allows(scope.membership.TenantID, resource, action)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrTenantDenied
	}
	return nil
}

type MenuItem struct {
	Key       string `json:"key"`
	CanRename bool   `json:"canRename"`
}

func (s *Service) Menu(scope TenantContext) ([]MenuItem, error) {
	if err := scope.require("menu", "view"); err != nil {
		return nil, err
	}
	domain := scope.membership.TenantID
	items := make([]MenuItem, 0, 1)
	read, err := scope.allows(domain, "departments", "read")
	if err != nil || !read {
		return items, err
	}
	rename, err := scope.allows(domain, "departments", "rename")
	if err != nil {
		return nil, err
	}
	return append(items, MenuItem{Key: "departments", CanRename: rename}), nil
}
