package tenant

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	service "github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlatformAPI struct {
	Operator uuid.UUID
	DB       func() *gorm.DB
}

func (api PlatformAPI) operatorClaims(c *gin.Context) (*request.CustomClaims, bool) {
	value, _ := c.Get("claims")
	claims, ok := value.(*request.CustomClaims)
	return claims, api.Operator != uuid.Nil && ok && claims != nil && claims.UUID == api.Operator
}

// BlockOperator keeps the dedicated platform account out of other private surfaces.
func (api PlatformAPI) BlockOperator(c *gin.Context) {
	if _, ok := api.operatorClaims(c); ok {
		fail(c, http.StatusForbidden, "平台账号只能访问平台接口")
	}
}

// Tenant returns platform metadata for a tenant.
// @Tags DealFlow Platform
// @Summary 平台运营账号查询租户元数据
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "租户 ID"
// @Success 200 {object} response.Response{data=tenant.Tenant}
// @Failure 400,401,403,404,500 {object} response.Response
// @Header all {string} X-Request-ID "服务端请求 ID"
// @Router /dealflow/platform/tenants/{id} [get]
func (api PlatformAPI) Tenant(c *gin.Context) {
	claims, ok := api.operatorClaims(c)
	if !ok {
		fail(c, http.StatusForbidden, "平台权限不足")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 || len(c.Request.Header.Values("X-Tenant-ID")) != 0 || c.Request.URL.RawQuery != "" {
		fail(c, http.StatusBadRequest, "平台请求参数无效")
		return
	}
	row, err := service.New(api.DB()).PlatformTenant(c.Request.Context(), api.Operator, claims.BaseClaims.ID, id)
	switch {
	case errors.Is(err, service.ErrTenantDenied):
		fail(c, http.StatusForbidden, "平台权限不足")
	case errors.Is(err, service.ErrTenantNotFound):
		fail(c, http.StatusNotFound, "租户不存在")
	case err != nil:
		fail(c, http.StatusInternalServerError, "平台操作失败")
	default:
		response.OkWithData(row, c)
	}
}
