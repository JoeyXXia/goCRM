package tenant

import (
	api "github.com/flipped-aurora/gin-vue-admin/server/api/v1/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(root *gin.RouterGroup, db func() *gorm.DB) {
	departments := api.DepartmentAPI{DB: db}
	group := root.Group("/dealflow/departments", api.RequestID, middleware.JWTAuth(),
		departments.ResolveTenant, middleware.CasbinHandler())
	group.GET("/:id", departments.Department)
	group.PATCH("/:id", departments.RenameDepartment)
}
