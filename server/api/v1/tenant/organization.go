package tenant

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	service "github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/gin-gonic/gin"
)

func organizationBody(c *gin.Context, target any) bool {
	body := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	body.DisallowUnknownFields()
	if err := body.Decode(target); err != nil || body.Decode(new(json.RawMessage)) != io.EOF {
		fail(c, http.StatusBadRequest, "组织参数无效")
		return false
	}
	return true
}

// Departments lists departments in the selected enterprise.
// @Tags DealFlow
// @Summary 查询当前企业部门列表
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Success 200 {object} response.Response{data=[]tenant.Department}
// @Failure 400,401,403,500 {object} response.Response
// @Router /dealflow/departments [get]
func (api DepartmentAPI) Departments(c *gin.Context) {
	req, ok := resolvedRequest(c)
	if !ok {
		return
	}
	rows, err := req.service.Departments(c.Request.Context(), req.scope)
	if err != nil {
		failService(c, err)
		return
	}
	response.OkWithData(rows, c)
}

// Ancestors returns the parent chain.
// @Tags DealFlow
// @Summary 查询部门祖先（直属父部门至根部门）
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "部门 ID"
// @Success 200 {object} response.Response{data=[]tenant.Department}
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/departments/{id}/ancestors [get]
func (api DepartmentAPI) Ancestors(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	rows, err := req.service.Ancestors(c.Request.Context(), req.scope, id)
	if err != nil {
		failService(c, err)
		return
	}
	response.OkWithData(rows, c)
}

// CreateDepartment creates a department.
// @Tags DealFlow
// @Summary 创建当前企业部门
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param input body tenant.DepartmentInput true "部门字段，parentId 为字符串或 null"
// @Success 200 {object} response.Response{data=tenant.Department}
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/departments [post]
func (api DepartmentAPI) CreateDepartment(c *gin.Context) { api.saveDepartment(c, false) }

// UpdateDepartment replaces department fields.
// @Tags DealFlow
// @Summary 更新当前企业部门及父部门
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "部门 ID"
// @Param input body tenant.DepartmentInput true "部门字段，省略 parentId 或 null 表示根部门"
// @Success 200 {object} response.Response{data=tenant.Department}
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/departments/{id} [put]
func (api DepartmentAPI) UpdateDepartment(c *gin.Context) { api.saveDepartment(c, true) }

func (api DepartmentAPI) saveDepartment(c *gin.Context, updating bool) {
	req, ok := resolvedRequest(c)
	var id uint64
	if updating {
		req, id, ok = departmentRequest(c)
	}
	if !ok {
		return
	}
	var input service.DepartmentInput
	if !organizationBody(c, &input) {
		return
	}
	row, err := req.service.SaveDepartment(c.Request.Context(), req.scope, id, input)
	if err != nil {
		failService(c, err)
		return
	}
	response.OkWithData(row, c)
}

// DeleteDepartment removes an empty leaf.
// @Tags DealFlow
// @Summary 删除无成员且无子部门的部门
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "部门 ID"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/departments/{id} [delete]
func (api DepartmentAPI) DeleteDepartment(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	if err := req.service.DeleteDepartment(c.Request.Context(), req.scope, id); err != nil {
		failService(c, err)
		return
	}
	response.Ok(c)
}

// Members lists tenant membership records.
// @Tags DealFlow
// @Summary 按当前角色的数据范围查询企业成员
// @Description 服务端合并角色允许范围并限制当前租户，拒绝查询参数。
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Success 200 {object} response.Response{data=[]tenant.Membership}
// @Failure 400,401,403,500 {object} response.Response
// @Router /dealflow/members [get]
func (api DepartmentAPI) Members(c *gin.Context) {
	req, ok := resolvedRequest(c)
	if !ok {
		return
	}
	rows, err := req.service.Members(c.Request.Context(), req.scope)
	if err != nil {
		failService(c, err)
		return
	}
	response.OkWithData(rows, c)
}

// CreateMember binds an existing enabled GVA account.
// @Tags DealFlow
// @Summary 绑定当前企业成员
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param input body tenant.MemberInput true "账号及部门 ID 使用字符串，dataRoles 可附加 sales、manager、finance"
// @Success 200 {object} response.Response{data=tenant.Membership}
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/members [post]
func (api DepartmentAPI) CreateMember(c *gin.Context) { api.saveMember(c, false) }

// UpdateMember changes membership attributes.
// @Tags DealFlow
// @Summary 更新成员归属角色和状态（active 或 disabled）
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "成员 ID"
// @Param input body tenant.MemberInput true "完整成员字段，userId 保持原账号，省略 dataRoles 清空附加数据角色"
// @Success 200 {object} response.Response{data=tenant.Membership}
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/members/{id} [put]
func (api DepartmentAPI) UpdateMember(c *gin.Context) { api.saveMember(c, true) }

func (api DepartmentAPI) saveMember(c *gin.Context, updating bool) {
	req, ok := resolvedRequest(c)
	var id uint64
	if updating {
		req, id, ok = departmentRequest(c)
	}
	if !ok {
		return
	}
	var input service.MemberInput
	if !organizationBody(c, &input) {
		return
	}
	row, err := req.service.SaveMember(c.Request.Context(), req.scope, id, input)
	if err != nil {
		failService(c, err)
		return
	}
	response.OkWithData(row, c)
}

// DeleteMember removes a membership.
// @Tags DealFlow
// @Summary 移除成员并保护最后一名活跃管理员
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "成员 ID"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/members/{id} [delete]
func (api DepartmentAPI) DeleteMember(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	if err := req.service.DeleteMember(c.Request.Context(), req.scope, id); err != nil {
		failService(c, err)
		return
	}
	response.Ok(c)
}
