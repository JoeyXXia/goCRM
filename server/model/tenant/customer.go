package tenant

type Customer struct {
	ID           uint64 `json:"id,string" swaggertype:"string"`
	TenantID     uint64 `json:"tenantId,string" swaggertype:"string"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	OwnerID      uint64 `json:"ownerId,string" swaggertype:"string"`
	DepartmentID uint64 `json:"departmentId,string" swaggertype:"string"`
	Industry     string `json:"industry"`
	Remark       string `json:"remark"`
}

func (Customer) TableName() string { return "crm_customers" }

type Contact struct {
	ID         uint64 `json:"id,string" swaggertype:"string"`
	TenantID   uint64 `json:"tenantId,string" swaggertype:"string"`
	CustomerID uint64 `json:"customerId,string" swaggertype:"string"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	Title      string `json:"title"`
}

func (Contact) TableName() string { return "crm_contacts" }
