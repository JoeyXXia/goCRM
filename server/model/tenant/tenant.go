package tenant

type Tenant struct {
	ID     uint64 `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Department struct {
	ID       uint64  `json:"id"`
	TenantID uint64  `json:"tenantId"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *uint64 `json:"parentId,string,omitempty" swaggertype:"string"`
}

type Membership struct {
	ID                uint64 `json:"id"`
	TenantID          uint64 `json:"tenantId"`
	UserID            uint64 `json:"userId"`
	DepartmentID      uint64 `json:"departmentId"`
	RoleKey           string `json:"roleKey"`
	Status            string `json:"status"`
	PermissionVersion uint64 `json:"permissionVersion" gorm:"default:1"`
}

func (Membership) TableName() string { return "tenant_memberships" }
