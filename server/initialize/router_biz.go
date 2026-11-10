package initialize

import (
	tenantapi "github.com/flipped-aurora/gin-vue-admin/server/api/v1/tenant"
	"github.com/flipped-aurora/gin-vue-admin/server/router"
	"github.com/flipped-aurora/gin-vue-admin/server/router/tenant"
	"github.com/gin-gonic/gin"
)

// 占位方法，保证文件可以正确加载，避免go空变量检测报错，请勿删除。
func holder(routers ...*gin.RouterGroup) {
	_ = routers
	_ = router.RouterGroupApp
}

func initBizRouter(platform tenantapi.PlatformAPI, routers ...*gin.RouterGroup) {
	privateGroup := routers[0]
	publicGroup := routers[1]

	holder(publicGroup, privateGroup)
	tenant.Register(publicGroup, platform)
}
