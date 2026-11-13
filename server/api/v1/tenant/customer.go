package tenant

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	service "github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"github.com/gin-gonic/gin"
)

func crmQuery(c *gin.Context, allowed ...string) (url.Values, bool) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		fail(c, http.StatusBadRequest, "查询参数无效")
		return nil, false
	}
	for key, items := range values {
		known := false
		for _, name := range allowed {
			if key == name {
				known = true
			}
		}
		if !known || len(items) != 1 {
			fail(c, http.StatusBadRequest, "查询参数无效")
			return nil, false
		}
	}
	return values, true
}

func crmResult(c *gin.Context, data any, err error) {
	if err != nil {
		failService(c, err)
		return
	}
	response.OkWithData(data, c)
}

// Customers lists only visible customers.
// @Tags DealFlow CRM
// @Summary 分页查询当前可见客户
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param page query int false "页码，默认 1，最大 1000000"
// @Param pageSize query int false "每页 1 至 100，默认 20"
// @Param sort query string false "id、name、code 或 nextFollowupAt，默认 id"
// @Param order query string false "asc 或 desc，默认 desc"
// @Param keyword query string false "客户名称包含文本，最大 128 字符"
// @Param followup query string false "none、overdue 或 scheduled，省略表示全部"
// @Success 200 {object} response.Response{data=tenant.CustomerPage}
// @Failure 400,401,403,500 {object} response.Response
// @Router /dealflow/customers [get]
func (api DepartmentAPI) Customers(c *gin.Context) {
	req, ok := resolvedRequest(c)
	if !ok {
		return
	}
	values, ok := crmQuery(c, "page", "pageSize", "sort", "order", "keyword", "followup")
	if !ok {
		return
	}
	input := service.CustomerSearch{Page: 1, PageSize: 20, Sort: "id", Order: "desc", Keyword: strings.TrimSpace(values.Get("keyword"))}
	input.Followup = values.Get("followup")
	for _, entry := range []struct {
		key    string
		target *int
	}{{"page", &input.Page}, {"pageSize", &input.PageSize}} {
		if value, exists := values[entry.key]; exists {
			number, err := strconv.Atoi(value[0])
			if err != nil {
				failService(c, service.ErrCRMInput)
				return
			}
			*entry.target = number
		}
	}
	if _, exists := values["sort"]; exists {
		input.Sort = values.Get("sort")
	}
	if _, exists := values["order"]; exists {
		input.Order = values.Get("order")
	}
	data, err := req.service.Customers(c.Request.Context(), req.scope, input)
	crmResult(c, data, err)
}

func crmIDs(c *gin.Context) ([]uint64, bool) {
	values, ok := crmQuery(c, "ids")
	if !ok {
		return nil, false
	}
	parts := strings.Split(values.Get("ids"), ",")
	if len(parts) == 0 || len(parts) > 100 {
		failService(c, service.ErrCRMInput)
		return nil, false
	}
	ids := make([]uint64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil || id == 0 {
			failService(c, service.ErrCRMInput)
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

// CustomerBatch applies the same visibility as the list.
// @Tags DealFlow CRM
// @Summary 批量查询可见客户
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param ids query string true "逗号分隔的客户 ID，最多 100 项"
// @Success 200 {object} response.Response{data=[]tenant.CustomerSummary}
// @Failure 400,401,403,500 {object} response.Response
// @Router /dealflow/customers/batch [get]
func (api DepartmentAPI) CustomerBatch(c *gin.Context) {
	req, ok := resolvedRequest(c)
	if !ok {
		return
	}
	ids, ok := crmIDs(c)
	if !ok {
		return
	}
	data, err := req.service.CustomerBatch(c.Request.Context(), req.scope, ids)
	crmResult(c, data, err)
}

// Customer returns per-record write controls.
// @Tags DealFlow CRM
// @Summary 查询客户详情及当前记录操作权限
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Success 200 {object} response.Response{data=tenant.CustomerDetail}
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/customers/{id} [get]
func (api DepartmentAPI) Customer(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	data, err := req.service.Customer(c.Request.Context(), req.scope, id)
	crmResult(c, data, err)
}

// CreateCustomer assigns tenant, owner, department and code on the server.
// @Tags DealFlow CRM
// @Summary 创建本人负责的客户
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param input body tenant.CustomerInput true "仅客户名称、行业与备注"
// @Success 200 {object} response.Response{data=tenant.Customer}
// @Failure 400,401,403,409,500 {object} response.Response
// @Router /dealflow/customers [post]
func (api DepartmentAPI) CreateCustomer(c *gin.Context) {
	req, ok := resolvedRequest(c)
	if !ok {
		return
	}
	var input service.CustomerInput
	if !strictBody(c, &input, 16384) {
		return
	}
	data, err := req.service.CreateCustomer(c.Request.Context(), req.scope, input)
	crmResult(c, data, err)
}

// UpdateCustomer accepts only descriptive fields.
// @Tags DealFlow CRM
// @Summary 更新可见且可编辑客户的名称行业和备注
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Param input body tenant.CustomerInput true "完整可编辑字段，省略可选字段将清空"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/customers/{id} [put]
func (api DepartmentAPI) UpdateCustomer(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	var input service.CustomerInput
	if !strictBody(c, &input, 16384) {
		return
	}
	crmResult(c, nil, req.service.UpdateCustomer(c.Request.Context(), req.scope, id, input))
}

// DeleteCustomer rejects deletion while contacts exist.
// @Tags DealFlow CRM
// @Summary 删除无联系人的可管理客户
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/customers/{id} [delete]
func (api DepartmentAPI) DeleteCustomer(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	crmResult(c, nil, req.service.DeleteCustomer(c.Request.Context(), req.scope, id))
}

// Contacts reads contacts through the parent customer.
// @Tags DealFlow CRM
// @Summary 查询可见客户的联系人
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Success 200 {object} response.Response{data=[]tenant.ContactView}
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/customers/{id}/contacts [get]
func (api DepartmentAPI) Contacts(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	data, err := req.service.Contacts(c.Request.Context(), req.scope, id)
	crmResult(c, data, err)
}

// CreateContact inherits its customer association.
// @Tags DealFlow CRM
// @Summary 为可编辑客户新增联系人
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "客户 ID"
// @Param input body tenant.ContactInput true "联系人名称电话邮箱和职位"
// @Success 200 {object} response.Response{data=tenant.Contact}
// @Failure 400,401,403,404,409,500 {object} response.Response
// @Router /dealflow/customers/{id}/contacts [post]
func (api DepartmentAPI) CreateContact(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	var input service.ContactInput
	if !strictBody(c, &input, 16384) {
		return
	}
	data, err := req.service.CreateContact(c.Request.Context(), req.scope, id, input)
	crmResult(c, data, err)
}

// Contact checks the parent even on a direct URL.
// @Tags DealFlow CRM
// @Summary 查询联系人并校验所属客户可见性
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "联系人 ID"
// @Success 200 {object} response.Response{data=tenant.ContactView}
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/contacts/{id} [get]
func (api DepartmentAPI) Contact(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	data, err := req.service.Contact(c.Request.Context(), req.scope, id)
	crmResult(c, data, err)
}

// ContactBatch filters by parent customer visibility.
// @Tags DealFlow CRM
// @Summary 批量查询可见客户的联系人
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param ids query string true "逗号分隔的联系人 ID，最多 100 项"
// @Success 200 {object} response.Response{data=[]tenant.ContactView}
// @Failure 400,401,403,500 {object} response.Response
// @Router /dealflow/contacts/batch [get]
func (api DepartmentAPI) ContactBatch(c *gin.Context) {
	req, ok := resolvedRequest(c)
	if !ok {
		return
	}
	ids, ok := crmIDs(c)
	if !ok {
		return
	}
	data, err := req.service.ContactBatch(c.Request.Context(), req.scope, ids)
	crmResult(c, data, err)
}

// UpdateContact cannot change its parent.
// @Tags DealFlow CRM
// @Summary 更新联系人可编辑字段
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "联系人 ID"
// @Param input body tenant.ContactInput true "完整可编辑字段，省略可选字段将清空"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/contacts/{id} [put]
func (api DepartmentAPI) UpdateContact(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	var input service.ContactInput
	if !strictBody(c, &input, 16384) {
		return
	}
	crmResult(c, nil, req.service.UpdateContact(c.Request.Context(), req.scope, id, input))
}

// DeleteContact requires parent write access.
// @Tags DealFlow CRM
// @Summary 删除可编辑客户的联系人
// @Security ApiKeyAuth
// @Produce json
// @Param X-Tenant-ID header string true "当前租户 ID"
// @Param id path string true "联系人 ID"
// @Success 200 {object} response.Response
// @Failure 400,401,403,404,500 {object} response.Response
// @Router /dealflow/contacts/{id} [delete]
func (api DepartmentAPI) DeleteContact(c *gin.Context) {
	req, id, ok := departmentRequest(c)
	if !ok {
		return
	}
	crmResult(c, nil, req.service.DeleteContact(c.Request.Context(), req.scope, id))
}
