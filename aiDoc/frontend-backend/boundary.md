# 前后端边界说明

## 归属边界

- 后端负责路由、参数校验、业务逻辑和响应结构
- 前端负责页面流程、交互体验、本地状态和展示层
- 共同行为通过明确的 API 契约协作，不通过隐式约定耦合

## 契约规则

- 保持统一响应结构：`{ code, data, msg }`
- 保持统一分页结构：`{ page, pageSize, total, list }`
- 字段名不要随意漂移
- 前后端字段类型必须保持一致
- 后端必须提供完整而准确的 Swagger 接口说明
- 前端接口调用应以实际 Swagger 与后端实现为准

## DealFlow 企业与平台接口

- 企业接口沿用 GVA 的 `x-token`。请求头 `X-Tenant-ID` 使用单个正整数 ID。服务端每次核对账号、租户及成员的当前状态，并在选中租户的域内执行 DealFlow 动作策略。
- `GET /dealflow/workspaces` 从 JWT 识别账号，只返回该账号当前启用的成员关系及企业。响应项包含 `id`、`name`、`departmentId`、`roleKey`，其中两个 ID 都是十进制字符串。该接口拒绝查询参数、`X-Tenant-ID` 和专用平台账号。
- `GET /dealflow/menu` 返回可见项 `[{ "key": "departments", "canRename": true }]`，其中 `canRename` 由当前成员角色决定。部门 API 在每次直接请求时执行动作校验。
- Vue 工作台在当前标签页只保存所选企业的 ID。刷新时重新发现成员关系并加载菜单。切换企业先清空菜单和详情，再取消旧请求。直接访问改名页面时，工作台先读取当前菜单能力，服务端仍在每次 PATCH 中验证动作权限。
- 部门操作使用 `GET /dealflow/departments/:id` 和 `PATCH /dealflow/departments/:id`。
- 路径 `:id` 表示部门 ID。接口不接收查询参数。PATCH 只接收 JSON 对象 `{"name":"部门名"}`，仅当前租户管理员可修改，名称去除首尾空白后须为 1 至 128 个字符。
- `GET /dealflow/platform/tenants/:id` 只允许配置的专用平台账号，返回租户的 `id`、`code`、`name` 和 `status`。该接口拒绝 `X-Tenant-ID` 和查询参数。服务启动时从 `DEALFLOW_PLATFORM_OPERATOR_UUID` 读取现有账号的 UUID。未配置有效 UUID 时平台入口拒绝访问。
- 保持 `{ code, data, msg }`。非本租户部门与不存在的部门同样返回 404，DealFlow 权限校验失败返回 403。GVA 继续为普通账号执行原有权限链路。专用平台账号访问企业接口或 GVA 私有路由时返回 403。DealFlow 响应头 `X-Request-ID` 由服务端生成。

## 变更规则

- 涉及破坏性接口调整时，要先写清楚变更范围
- Swagger 或其他接口说明必须与真实实现一致
- 前端接口封装应继续放在 `web/src/api/` 或 `web/src/plugin/<name>/api/`
- 可复用逻辑优先复用 `web/src/utils/` 现有能力

## 完成前检查

跨前后端改动结束前，至少确认以下几点：

1. 后端响应结构仍然满足前端预期
2. 前端仍在使用正确的字段名和数据类型
3. 若契约发生了长期变化，对应说明已经补到 `aiDoc/`
