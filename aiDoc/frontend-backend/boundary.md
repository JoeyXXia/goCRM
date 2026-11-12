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

## Day 8 组织管理接口

- 部门列表使用 `GET /dealflow/departments`，父子关系通过 `parentId` 表达。创建使用 POST，完整更新使用 `PUT /dealflow/departments/:id`，删除使用 DELETE。请求字段为 `code`、`name`、`parentId`，父部门 ID 使用十进制字符串，省略或 null 表示根部门。祖先查询使用 `GET /dealflow/departments/:id/ancestors`，按直属父部门到根部门返回。
- 企业成员通过 `GET /dealflow/members` 查询当前数据范围内的成员。企业管理员通过 `POST /dealflow/members` 和 `PUT/DELETE /dealflow/members/:id` 管理成员。成员创建和更新使用完整字段 `userId`、`departmentId`、`roleKey`、`status`，两个输入 ID 使用字符串。更新时账号 ID 保持一致。角色可取 `tenant_admin`、`sales`、`manager`、`finance`，状态可取 `active`、`disabled`。
- 成员记录新增 `permissionVersion`，初值为 1。成员部门、角色或状态变化时递增，幂等更新保持版本。每次企业请求重新检查当前成员状态，旧 JWT 在成员禁用或移除后访问该企业返回 403。
- 组织接口继承企业请求头和统一响应约定。部门与成员响应 ID 延续后端模型的整数格式，新字段 `parentId` 使用字符串或省略。树环、删除有依赖部门、重复成员和移除最后一名活跃管理员返回 409。跨企业资源与不存在的资源统一返回 404。

## Day 9 数据范围

- `GET /dealflow/members` 按当前租户内的角色授予记录范围。管理员为全部，销售为本人，财务为本部门，经理为本部门及后代部门。成员记录以 `userId` 作为本人判定字段，以 `departmentId` 判断部门归属。
- 创建与完整更新成员可携带 `dataRoles` 字符串数组，允许附加 `sales`、`manager`、`finance`。同动作的允许范围取并集，最外层始终限制可信租户。额外数据角色由管理员写入，组织写权限继续使用主角色。省略该字段清空附加角色，角色集合排序并去重，变化时递增 `permissionVersion`。
- 客户端提供的角色、范围或部门列表不会成为读权限来源。查询参数继续被拒绝，GET 请求体不参与范围解析。每次读取重新获取当前成员及权限版本，数据过滤结果限本次请求使用。规则详见 [数据范围 ADR](../../docs/adr/0003-record-visibility.md)。

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
