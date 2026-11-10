package tenant

import (
	api "github.com/flipped-aurora/gin-vue-admin/server/api/v1/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

func Register(root *gin.RouterGroup, platform api.PlatformAPI) {
	dealflow := root.Group("/dealflow", api.RequestID, middleware.JWTAuth())
	departments := api.DepartmentAPI{DB: platform.DB}
	enterprise := dealflow.Group("", platform.BlockOperator, departments.ResolveTenant)
	enterprise.GET("/menu", departments.Menu)
	enterprise.GET("/departments/:id", departments.Department)
	enterprise.PATCH("/departments/:id", departments.RenameDepartment)
	dealflow.Group("/platform").GET("/tenants/:id", platform.Tenant)
}
