package tenant

import (
	api "github.com/flipped-aurora/gin-vue-admin/server/api/v1/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

func Register(root *gin.RouterGroup, platform api.PlatformAPI) {
	dealflow := root.Group("/dealflow", api.RequestID, middleware.JWTAuth())
	departments := api.DepartmentAPI{DB: platform.DB}
	account := dealflow.Group("", platform.BlockOperator)
	account.GET("/workspaces", departments.Workspaces)
	enterprise := account.Group("", departments.ResolveTenant)
	enterprise.GET("/menu", departments.Menu)
	enterprise.GET("/departments/:id", departments.Department)
	enterprise.PATCH("/departments/:id", departments.RenameDepartment)
	enterprise.GET("/departments", departments.Departments)
	enterprise.POST("/departments", departments.CreateDepartment)
	enterprise.PUT("/departments/:id", departments.UpdateDepartment)
	enterprise.DELETE("/departments/:id", departments.DeleteDepartment)
	enterprise.GET("/departments/:id/ancestors", departments.Ancestors)
	enterprise.GET("/members", departments.Members)
	enterprise.POST("/members", departments.CreateMember)
	enterprise.PUT("/members/:id", departments.UpdateMember)
	enterprise.DELETE("/members/:id", departments.DeleteMember)
	query := account.Group("", departments.ResolveTenantQuery)
	query.GET("/customers", departments.Customers)
	query.GET("/customers/batch", departments.CustomerBatch)
	query.GET("/contacts/batch", departments.ContactBatch)
	query.GET("/customers/:id/followups", departments.Followups)
	enterprise.POST("/customers", departments.CreateCustomer)
	enterprise.GET("/customers/:id", departments.Customer)
	enterprise.PUT("/customers/:id", departments.UpdateCustomer)
	enterprise.DELETE("/customers/:id", departments.DeleteCustomer)
	enterprise.POST("/customers/:id/followups", departments.AddFollowup)
	enterprise.GET("/customers/:id/transfer-targets", departments.TransferTargets)
	enterprise.POST("/customers/:id/transfer", departments.TransferCustomer)
	enterprise.GET("/customers/:id/audit", departments.CustomerAudit)
	enterprise.GET("/customers/:id/contacts", departments.Contacts)
	enterprise.POST("/customers/:id/contacts", departments.CreateContact)
	enterprise.GET("/contacts/:id", departments.Contact)
	enterprise.PUT("/contacts/:id", departments.UpdateContact)
	enterprise.DELETE("/contacts/:id", departments.DeleteContact)
	dealflow.Group("/platform").GET("/tenants/:id", platform.Tenant)
}
