package tenant

import (
	"strconv"

	service "github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/gin-gonic/gin"
)

// Followups reads the timeline through the visible customer.
// @Tags DealFlow CRM
// @Summary 分页查询客户跟进时间线
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Param page query int false "页码，默认 1"
// @Param pageSize query int false "每页 1 至 100，默认 20"
// @Success 200 {object} response.Response{data=tenant.FollowupPage}
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/customers/{id}/followups [get]
func (api DepartmentAPI) Followups(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	values, ok := crmQuery(c, "page", "pageSize")
	if !ok {
		return
	}
	page, size := 1, 20
	for _, entry := range []struct {
		key   string
		value *int
	}{{"page", &page}, {"pageSize", &size}} {
		if raw, exists := values[entry.key]; exists {
			n, err := strconv.Atoi(raw[0])
			if err != nil {
				failService(c, service.ErrCRMInput)
				return
			}
			*entry.value = n
		}
	}
	result, err := req.service.Followups(c.Request.Context(), req.scope, id, page, size)
	crmResult(c, result, err)
}

// AddFollowup appends a note and changes the next followup in one transaction.
// @Tags DealFlow CRM
// @Summary 新增客户跟进并设置下次跟进时间
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Param input body tenant.FollowupInput true "nextFollowupAt 使用 RFC3339，省略或 null 清除"
// @Success 200 {object} response.Response{data=tenant.Followup}
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/customers/{id}/followups [post]
func (api DepartmentAPI) AddFollowup(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	var input service.FollowupInput
	if !strictBody(c, &input, 16384) {
		return
	}
	result, err := req.service.AddFollowup(c.Request.Context(), req.scope, id, input)
	crmResult(c, result, err)
}

// TransferTargets lists eligible recipients in the primary manager's range.
// @Tags DealFlow CRM
// @Summary 查询客户可转移的启用负责人
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Success 200 {object} response.Response{data=[]tenant.TransferTarget}
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/customers/{id}/transfer-targets [get]
func (api DepartmentAPI) TransferTargets(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	result, err := req.service.TransferTargets(c.Request.Context(), req.scope, id)
	crmResult(c, result, err)
}

// TransferCustomer changes owner and department from the target's membership.
// @Tags DealFlow CRM
// @Summary 主角色为管理员或经理时受控转移客户
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Param input body tenant.TransferInput true "目标负责人账号 ID"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/customers/{id}/transfer [post]
func (api DepartmentAPI) TransferCustomer(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	var input service.TransferInput
	if !strictBody(c, &input, 4096) {
		return
	}
	crmResult(c, nil, req.service.TransferCustomer(c.Request.Context(), req.scope, id, input))
}

// CustomerAudit exposes identifier-only audit events to the primary manager.
// @Tags DealFlow CRM
// @Summary 查询可管理客户最近 100 条审计事件
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Success 200 {object} response.Response{data=[]tenant.AuditView}
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/customers/{id}/audit [get]
func (api DepartmentAPI) CustomerAudit(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	result, err := req.service.CustomerAudit(c.Request.Context(), req.scope, id)
	crmResult(c, result, err)
}
