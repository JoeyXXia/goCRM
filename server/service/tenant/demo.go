package tenant

import (
	"context"
	"errors"
	"fmt"
	"sort"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DemoUsers struct {
	Admin   uint64
	Sales   uint64
	Manager uint64
	Finance uint64
}

// SeedDemo binds explicitly selected accounts; it never creates or changes GVA credentials.
func (s *Service) SeedDemo(ctx context.Context, users DemoUsers) error {
	ids := []uint64{users.Admin, users.Sales, users.Manager, users.Finance}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		if id == 0 || (i > 0 && id == ids[i-1]) {
			return errors.New("demo requires four distinct GVA user IDs")
		}
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			if err := checkUser(tx, id); err != nil {
				return err
			}
		}
		for _, fixture := range []struct {
			code, name     string
			sales, manager uint64
		}{
			{"demo-a", "晨星设备", users.Sales, users.Manager},
			{"demo-b", "远舟服务", users.Manager, users.Sales},
		} {
			input := ProvisionInput{Code: fixture.code, Name: fixture.name, AdminUserID: users.Admin}
			members := []member{{users.Admin, administrator}, {fixture.sales, "sales"}, {fixture.manager, "manager"}, {users.Finance, "finance"}}
			if err := ensureDemoTenant(tx, input, members); err != nil {
				return fmt.Errorf("seed %s: %w", fixture.code, err)
			}
		}
		return nil
	})
}

func ensureDemoTenant(tx *gorm.DB, input ProvisionInput, members []member) error {
	var tenant model.Tenant
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", input.Code).Take(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_, err := createTenant(tx, input, members)
		return err
	}
	if err != nil {
		return err
	}
	if tenant.Name != input.Name || tenant.Status != active {
		return errors.New("existing tenant differs from demo seed")
	}

	var department model.Department
	if err := tx.Where("tenant_id = ? AND code = ?", tenant.ID, defaultDepartment).Take(&department).Error; err != nil {
		return fmt.Errorf("find demo department: %w", err)
	}
	if department.Name != "综合部" {
		return errors.New("existing department differs from demo seed")
	}
	var bindings []model.Membership
	if err := tx.Where("tenant_id = ?", tenant.ID).Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) != len(members) {
		return errors.New("existing membership count differs from demo seed")
	}
	expected := make(map[uint64]string, len(members))
	for _, m := range members {
		expected[m.userID] = m.role
	}
	for _, binding := range bindings {
		role, ok := expected[binding.UserID]
		if !ok || role != binding.RoleKey || binding.DepartmentID != department.ID || binding.Status != active || len(binding.DataRoles) != 0 {
			return errors.New("existing membership differs from demo seed")
		}
	}
	return nil
}
