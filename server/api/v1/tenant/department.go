package tenant

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	service "github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const requestTenantKey = "dealflow.tenant"

type DepartmentAPI struct {
	DB func() *gorm.DB
}

type tenantRequest struct {
	service *service.Service
	scope   service.TenantContext
}

type RenameDepartmentInput struct {
	Name string `json:"name" binding:"required" maxLength:"128"`
}

func RequestID(c *gin.Context) {
	c.Header("X-Request-ID", uuid.NewString())
}

func (api DepartmentAPI) ResolveTenant(c *gin.Context) {
	value, _ := c.Get("claims")
	claims, ok := value.(*request.CustomClaims)
	if !ok || claims == nil || claims.BaseClaims.ID == 0 {
		fail(c, http.StatusUnauthorized, "登录身份无效")
		return
	}
	ids := c.Request.Header.Values("X-Tenant-ID")
	if len(ids) != 1 {
		fail(c, http.StatusBadRequest, "需要一个 X-Tenant-ID 请求头")
		return
	}
	id, err := strconv.ParseUint(ids[0], 10, 64)
	if err != nil || id == 0 || c.Request.URL.RawQuery != "" {
		fail(c, http.StatusBadRequest, "租户选择或请求参数无效")
		return
	}
	svc := service.New(api.DB())
	scope, err := svc.ResolveTenant(c.Request.Context(), uint64(claims.BaseClaims.ID), id)
	if err != nil {
		failService(c, err)
		return
	}
	c.Set(requestTenantKey, tenantRequest{service: svc, scope: scope})
}

// Department returns a department in the selected tenant.
// @Tags DealFlow
// @Summary 查询当前租户内的部门
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "部门 ID"
// @Success 200 {object} response.Response{data=tenant.Department}
// @Failure 400,401,403,404,500 {object} response.Response
// @Header all {string} X-Request-ID "服务端请求 ID"
// @Router /dealflow/departments/{id} [get]
func (api DepartmentAPI) Department(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	row, err := req.service.Department(c.Request.Context(), req.scope, id)
	if err != nil {
		failService(c, err)
		return
	}
	response.OkWithData(row, c)
}

// RenameDepartment updates only the name of a department in the selected tenant.
// @Tags DealFlow
// @Summary 当前租户管理员修改部门名称
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "部门 ID"
// @Param input body RenameDepartmentInput true "部门名称"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,500 {object} response.Response
// @Header all {string} X-Request-ID "服务端请求 ID"
// @Router /dealflow/departments/{id} [patch]
func (api DepartmentAPI) RenameDepartment(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	var input RenameDepartmentInput
	body := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	body.DisallowUnknownFields()
	if err := body.Decode(&input); err != nil || body.Decode(new(json.RawMessage)) != io.EOF {
		fail(c, http.StatusBadRequest, "请求体只能包含部门名称")
		return
	}
	if err := req.service.RenameDepartment(c.Request.Context(), req.scope, id, input.Name); err != nil {
		failService(c, err)
		return
	}
	response.Ok(c)
}

func departmentRequest(c *gin.Context) (tenantRequest, uint64, bool) {
	state, _ := c.Get(requestTenantKey)
	req, ok := state.(tenantRequest)
	if !ok {
		failService(c, service.ErrTenantDenied)
		return req, 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		fail(c, http.StatusBadRequest, "部门 ID 无效")
		return req, 0, false
	}
	return req, id, true
}

func failService(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTenantDenied):
		fail(c, http.StatusForbidden, "租户权限不足")
	case errors.Is(err, service.ErrDepartmentNotFound):
		fail(c, http.StatusNotFound, "部门不存在")
	case errors.Is(err, service.ErrInvalidName):
		fail(c, http.StatusBadRequest, "部门名称须为 1 至 128 个字符")
	default:
		fail(c, http.StatusInternalServerError, "租户操作失败")
	}
}

func fail(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, response.Response{Code: response.ERROR, Data: nil, Msg: message})
}
