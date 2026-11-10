package tenant

import (
	"testing"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
)

func TestEnterprisePolicyDomainAndDefaultDeny(t *testing.T) {
	const domain = 1<<63 + 1
	scope := TenantContext{membership: model.Membership{ID: 1, UserID: 1, TenantID: domain, RoleKey: administrator}}
	for _, tc := range []struct {
		name     string
		domain   uint64
		resource string
		action   string
		want     bool
	}{
		{"own tenant rename", domain, "departments", "rename", true},
		{"adjacent tenant rename", domain + 1, "departments", "rename", false},
		{"unmapped customer read", domain, "customers", "read", false},
		{"platform metadata", domain, "platform.tenants", "read", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scope.allows(tc.domain, tc.resource, tc.action)
			if err != nil || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, nil)", got, err, tc.want)
			}
		})
	}
	scope.membership.RoleKey = "admin"
	if allowed, err := scope.allows(domain, "menu", "view"); err != nil || allowed {
		t.Fatalf("unmapped admin alias gave (%v, %v)", allowed, err)
	}
}
