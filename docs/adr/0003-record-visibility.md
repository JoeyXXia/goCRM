# ADR 0003 · 按动作合并数据范围

2026-11-12 · Day 9

## 决策

Casbin 的企业策略增加数据范围字段。服务端根据当前成员的主角色与附加数据角色，收集目标资源、目标动作的允许范围，取并集后与可信租户条件相交。

```text
tenant_id = 当前租户
AND (
    owner_id = 当前账号
    OR department_id = 当前部门
    OR department_id IN 当前部门及其后代
)
```

ALL 作为范围并集的全集，保留外层租户限制。动作没有允许范围、上下文缺失或成员已停用时拒绝访问。部门后代查询先限制租户，再遍历部门树，发现循环时返回错误。

## 当前动作的默认映射

本轮为成员读取动作选择以下映射。其他业务动作通过独立策略定义。

| 角色 | members/read 数据范围 |
| --- | --- |
| tenant_admin | ALL |
| sales | SELF |
| finance | DEPARTMENT |
| manager | DEPARTMENT_AND_CHILDREN |

管理员可在成员的 `dataRoles` 数组附加 sales、finance、manager。只有主角色为 tenant_admin 的成员可以操作组织管理接口。附加数据角色参与数据范围合并。字段存放于成员记录的 JSON 列，既有成员的空值使用原主角色规则。

| 记录归属 | SELF | DEPARTMENT | DEPARTMENT_AND_CHILDREN | ALL | SELF 与 DEPARTMENT 并集 |
| --- | --- | --- | --- | --- | --- |
| 同租户、本人、本部门 | 允许 | 允许 | 允许 | 允许 | 允许 |
| 同租户、他人、本部门 | 拒绝 | 允许 | 允许 | 允许 | 允许 |
| 同租户、他人、下级部门 | 拒绝 | 拒绝 | 允许 | 允许 | 拒绝 |
| 同租户、他人、兄弟部门 | 拒绝 | 拒绝 | 拒绝 | 允许 | 拒绝 |
| 同租户、本人、兄弟部门 | 允许 | 拒绝 | 拒绝 | 允许 | 允许 |
| 异租户、同负责人或同部门编号 | 拒绝 | 拒绝 | 拒绝 | 拒绝 | 拒绝 |

## 查询与版本

`ResolveVisibility` 通过可信上下文中的账号和租户重新查询当前成员，并核对成员 ID。它使用当前权限版本、角色、状态和部门。客户端的范围、角色或部门列表不参与读取授权。

`Visibility.ForRecords` 为具有 tenant_id、owner_id、department_id 的模型生成 GORM 查询条件。GORM scope 在执行查询时收集业务过滤条件，把其中的 OR 表达式整体加括号，再与租户及范围条件相交。现有成员列表通过相同过滤器使用 user_id 作为本人字段。客户和联系人功能按 Day 10 计划接入各自的资源动作。

成员的部门、主角色、状态或附加数据角色变更，在同一事务递增 permission_version。附加角色排序去重后相同的更新保持版本。读取时重新解析当前状态，过滤结果限本次请求使用。本轮的数据范围结果直接查询数据库。

已开始执行的请求持有本次解析的范围。下一次请求会读取新版本。部门移动后的下一次后代解析也会读取当前部门树。

源码见 [范围解析与查询](../../server/service/tenant/visibility.go)、[企业策略](../../server/service/tenant/policy.go)、[成员写入](../../server/service/tenant/organization.go)与 [真实数据库回归](../../server/cmd/dealflow-migrate/tenant_context_test.go)。
