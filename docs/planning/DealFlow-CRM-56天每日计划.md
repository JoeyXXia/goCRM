# DealFlow CRM：56 天每日执行清单

2026-11-13 当前本地验收范围为 Day 11。实际结果见 [Day 11 报告](../progress/2026-11-13-day11.md)。套餐、工作流、异步服务、Kafka 与 Kubernetes 保留于 [扩展实施计划](./DealFlow-CRM-扩展实施计划.md)。

配套：[项目总计划](./DealFlow-CRM-项目总计划.md) · [面试与 AI 协作手册](./DealFlow-CRM-面试与AI协作手册.md)。

技术：Go / Gin / GVA / GORM / MySQL 8 / Redis；Vue 3 / TypeScript / Element Plus / Pinia；Go testing / httptest / Vitest / Playwright；Docker Compose / Nginx / GitHub Actions。

按已有后端基础、每天约 4 小时设计，每周第 7 天约 1 小时复盘或休息。Go/Vue 都陌生时先加基础周或延长周期。日期从实际开工算起，不伪造提交时间。以下“验收”均为未来要执行的检查，不代表已经通过。

每天：15 分钟明确 Issue 和验收，约 100 分钟 AI 辅助分步实现，60 分钟亲自验证与审查，30 分钟独立学习，余下时间整理提交。建议 2–4 个真实有效提交，提交建议只是可选拆分，不是硬性数量。

T01–T16 对应总计划的自动化验收矩阵。新增高风险业务当天补测试；第 7 周做系统性查漏，不把测试全部拖到最后。每周结束仍有隔离/金额错误，先修复再做新模块。

## 第 1 周：读懂 GVA，建立租户与权限底座

### Day 1 — 范围与可复现的 GVA 基线

- **实现**：创建独立项目仓库；固定 GVA tag/SHA、工具链和锁文件，运行原始前后端；写业务故事、P0 边界、UPSTREAM 与 `.env.example`，保留适用上游声明。
- **AI 协作**：让 AI 画当前版本登录→JWT→路由→权限→数据库调用链，逐个打开源码验证。
- **验收**：干净环境按记录可启动；没有真实秘密入库；能区分上游功能和计划新增内容。
- **提交建议**：`chore: establish pinned GVA baseline`；`docs: define SaaS scope and upstream attribution`。
- **面试主题**：Go 包/module、GVA 的职责分层、为什么采用模块化单体。

### Day 2 — 数据库迁移与测试环境

- **实现**：Compose MySQL/必要 Redis；独立测试库；首个版本化迁移与测试数据清理；Go httptest 骨架、前端 typecheck、基础 CI。
- **AI 协作**：AI 生成脚本，你检查测试 DB 名校验、防误删和 CI service readiness。
- **验收**：空库迁移成功；两次运行测试无残留影响；非法配置失败清晰；上游基础登录可回归。
- **提交建议**：`chore: add versioned migrations and test database`；`ci: validate Go and Vue baseline`。
- **面试主题**：GORM AutoMigrate 与版本迁移、Go error 与 defer。

### Day 3 — 租户、成员与演示身份

- **实现**：tenants、memberships、departments 基础表；关联 GVA 用户；seed A/B 企业及销售/经理/财务；平台手动建租户与首管理员的事务服务。
- **AI 协作**：AI 提议关系模型，你确认角色不能仅挂全局用户而忽略企业。
- **验收**：同用户可有独立企业成员关系；重复成员受唯一键限制；建租户中途失败无半成品。
- **提交建议**：`feat: model tenant memberships`；`feat: seed isolated demo enterprises`；`test: verify tenant provisioning rollback`。
- **面试主题**：用户、成员、部门和角色的区别；复合唯一键。

### Day 4 — 可信 TenantContext

- **实现**：服务端验证当前租户成员、状态；业务 Repository 必须接收 TenantContext；缺失默认拒绝；统一错误与 requestID；查询/写入 tenant 白名单。
- **AI 协作**：让 AI 列出 query/body/header 中伪造 tenant 的请求，你亲自运行。
- **验收**：T01 初版：A token 请求 B 数据无结果且不改 B；缺 TenantContext 不能变成全表查询。
- **提交建议**：`feat: enforce trusted tenant context`；`test: reject forged tenant identifiers`；`feat: standardize request errors`。
- **面试主题**：认证与授权、context.Context 的用途与取消传播。

### Day 5 — Casbin 域与平台边界

- **实现**：企业业务独立策略 namespace/model，tenant domain 绑定角色；平台和企业 API 分组；角色动作映射；保留上游系统权限链路。
- **AI 协作**：AI 起草策略矩阵，你检查同名 admin 在 A/B 企业及平台是否错误继承。
- **验收**：T04 初版：平台凭据不能直接读客户；企业管理员不能操作平台租户；菜单和 API 各有校验。
- **提交建议**：`feat: scope business roles by tenant domain`；`test: separate platform and enterprise access`。
- **面试主题**：RBAC、Casbin matcher/domain；菜单权限为什么不够。

### Day 6 — 前端身份上下文与首次隔离回归

- **实现**：Vue/TS 会话 store、企业切换、角色工作台壳；切换租户清理列表/详情/权限状态并取消旧请求；后端复合关联约束的测试样例。
- **AI 协作**：AI 写切租户流程，你模拟旧请求晚到，检查不会把 A 的结果渲染到 B。
- **验收**：T01/T02/T04 基础 CI 通过；无权限直接输入 URL 仍被拒绝；刷新恢复正确企业。
- **提交建议**：`feat: add tenant-aware workspace shell`；`fix: discard stale responses after tenant switch`；`test: guard cross-tenant associations`。
- **面试主题**：Pinia 会话状态、请求取消、前后端职责边界。

### Day 7 — 轻量复盘与 v0.1

- **实现**：整理身份链路与隔离 ADR，修复第一周遗留；录一段两租户隔离视频，满足门槛才打 v0.1。
- **AI 协作**：让 AI 只提追问，你脱离生成代码解释 TenantContext 从哪里来。
- **验收**：能追踪一次请求的可信信息来源；没有“tenant_id 由前端决定”的路径。
- **提交建议**：`docs: document tenant trust boundaries`；必要时独立 `fix:` 提交。
- **面试主题**：共享库共享表与独立数据库隔离的取舍。

## 第 2 周：组织、数据范围与 CRM

### Day 8 — 部门树与成员管理

- **实现**：部门 CRUD、成员增禁用与归属；部门祖先查询；禁止树环、跨租户部门和移除最后管理员；加入成员权限版本字段。
- **AI 协作**：AI 生成祖先/后代用例，你检查移动部门时循环引用。
- **验收**：不同企业同名部门互不影响；禁用成员后新请求失败；旧 token 不能绕过成员状态。
- **提交建议**：`feat: add tenant departments and membership lifecycle`；`test: reject invalid department and owner changes`。
- **面试主题**：树结构建模、递归查询、Go slice/map。

### Day 9 — 数据范围与授权测试

- **实现**：SELF/部门/含子部门/全部四级范围；同动作允许范围取并集后 AND tenant；只允许服务端解析，记录规则 ADR。
- **AI 协作**：AI 画权限真值表，你从“多角色加起来是否越租户”检查。
- **验收**：T03 参数化测试覆盖同部门、下级、兄弟部门、异租户；权限修改后按新版本评估。
- **提交建议**：`feat: resolve record visibility by role scope`；`test: cover department visibility matrix`。
- **面试主题**：动作权限与记录权限、AND/OR 组合错误。

### Day 10 — 客户与联系人

- **实现**：客户/联系人 API 和 Vue 页面；业务编号、负责人、部门、联系人；参数分页、排序白名单、更新字段白名单；联系人跟随客户可见性。
- **AI 协作**：允许 AI 生成 CRUD 外壳，你逐个补 tenant、owner、关联校验和可编辑字段。
- **验收**：T01/T02 扩到详情、更新、批量查询；普通销售不能通过提交 owner/tenant 获得不应有权限。
- **提交建议**：`feat: manage scoped customers and contacts`；`test: restrict customer fields and associations`。
- **面试主题**：DTO 与模型分离、批量赋值漏洞、索引前缀。

### Day 11 — 跟进与客户转移

- **实现**：跟进记录、下一次跟进时间、时间线；受控客户负责人转移；审计骨架与私密字段脱敏；列表筛选与空态。
- **AI 协作**：AI 写转移场景，你检查旧销售能否继续访问联系人的直接 URL。
- **验收**：客户转移后旧负责人失去后续访问，新负责人可见；跟进不能挂在 B 客户；成功变更有审计。
- **提交建议**：`feat: track customer followups and ownership`；`test: re-evaluate access after ownership transfer`。
- **面试主题**：根对象授权、审计与普通日志。

### Day 12 — 商机管道

- **实现**：商机阶段、预计金额、预计关闭日、赢单/输单与原因；阶段历史；表格/简单阶段看板，避免拖拽大工程。
- **AI 协作**：AI 枚举状态转换，你确认赢单不自动虚增销售额或回款。
- **验收**：商机归属正确；非法阶段转换拒绝；历史可回看；筛选条件与分页一致。
- **提交建议**：`feat: add opportunity stages and history`；`test: enforce opportunity transitions`。
- **面试主题**：状态机、金额序列化、SQL 查询条件组合。

### Day 13 — CRM 工作台与浏览器回归

- **实现**：我的客户、待跟进、商机统计；Vue 表单验证、loading/error/retry；Playwright 覆盖 A 创建、B 不可见和客户转移。
- **AI 协作**：AI 起草用户行为测试，你去除脆弱的 CSS 选择器与仅断言页面文本的越权测试。
- **验收**：T01/T03 同时有 API 断言和浏览器链路；不存在只在前端做范围过滤的列表。
- **提交建议**：`feat: complete sales workspace`；`test: exercise CRM access in browser`。
- **面试主题**：Vue computed/watch、组件与 E2E 测试分工。

### Day 14 — 轻量复盘与 v0.2

- **实现**：维护客户/商机 ER 和权限矩阵，整理一个真实越权缺陷及修复记录；审查生成代码仍有的旁路。
- **AI 协作**：AI 以审查者身份找 Preload、JOIN、全局 DB 和批量更新遗漏。
- **验收**：CRM 主流程可用；本周新增端点全部归入权限测试矩阵。
- **提交建议**：`docs: record CRM access model and review findings`。
- **面试主题**：如何证明不是只给 CRUD 加了 tenant_id。

## 第 3 周：合同版本、审批与订单应收

### Day 15 — 金额与合同版本模型

- **实现**：统一分单位、API 金额字符串、输入上限/溢出校验；contracts/revisions；从商机创建合同草稿；草稿编辑带 expectedVersion。
- **AI 协作**：AI 给金额解析测试，你亲算 0.01、超长输入、负值和边界。
- **验收**：不接受浮点近似金额；并发编辑不会静默覆盖；跨客户/企业关联拒绝。
- **提交建议**：`feat: add exact money and contract revisions`；`test: guard amount boundaries and stale edits`。
- **面试主题**：int64、JSON 大整数、乐观锁与 RowsAffected。

### Day 16 — 审批状态机与版本冻结

- **实现**：提交、通过、驳回；单审批人、禁止自审；提交冻结 revision；驳回后复制成新版本；决定和审计同事务。
- **AI 协作**：AI 写状态表，你审查每个入口是否都绑定正确 revision_id。
- **验收**：T06：旧版本审批失败、待审批不能改金额、两次审批最多一个有效决定。
- **提交建议**：`feat: approve immutable contract revisions`；`test: prevent stale and duplicate approval`。
- **面试主题**：业务状态机、并发审批、版本快照。

### Day 17 — 审批前端与审计时间线

- **实现**：合同详情、版本金额对照、提交/审批意见、我的待办、历史；页面按服务器给出的状态/能力显示动作，但后端再次校验。
- **AI 协作**：AI 完成表单和状态组件，你模拟多人同时打开页面的过期状态。
- **验收**：过期审批返回可理解的冲突并刷新；销售无法手动调用审批接口；审计不泄漏其他企业。
- **提交建议**：`feat: add contract review workspace`；`test: handle stale approval actions in UI`。
- **面试主题**：客户端缓存与权威状态、409 错误处理。

### Day 18 — 合同私有附件

- **实现**：私有存储卷、文件元数据与授权下载；类型/大小/随机路径控制；合同版本保存附件引用；待审批版本不允许替换附件。
- **AI 协作**：AI 提供上传辅助代码，你检查上游默认静态目录是否会绕过权限。
- **验收**：T05 初版：复制文件 URL 到 B 或无登录会话不可下载；无目录穿越；旧版本附件不被覆盖。
- **提交建议**：`feat: secure contract attachments by resource access`；`test: block cross-tenant file access`。
- **面试主题**：私有文件授权、MIME、业务版本与文件不可变性。

### Day 19 — 订单确认与合同额度

- **实现**：订单及明细、草稿/确认；锁合同父行校验有效订单累计；明确单价数量计算；幂等命令基础和请求摘要。
- **AI 协作**：AI 写并发调用器，你检查使用真正并发而非循环串行请求。
- **验收**：T07/T08 初版：两个订单抢剩余额度不能超额；同键不同内容 409；同键重试不多一张订单。
- **提交建议**：`feat: confirm orders within approved contract amount`；`test: verify concurrent contract allocation`。
- **面试主题**：行锁、事务隔离、幂等与业务唯一键。

### Day 20 — 应收计划与事务事件

- **实现**：确认订单同事务生成分期应收、审计、outbox；金额合计一致；固定日期与取整规则；无回款历史订单取消释放额度。
- **AI 协作**：AI 写故障注入测试，你确认所有步骤传同一 tx，外部操作不在事务内。
- **验收**：订单/应收/审计/outbox 任一步失败整体回滚；应收合计等于订单金额；重复取消不重复释放。
- **提交建议**：`feat: create receivables with order confirmation`；`test: rollback order workflow atomically`。
- **面试主题**：事务边界、outbox、GORM tx 句柄混用风险。

### Day 21 — 轻量复盘与 v0.3

- **实现**：录制商机→合同审批→订单应收；补状态图、锁顺序 ADR 和 API 示例。
- **AI 协作**：AI 提问“审批完怎么防改金额”，你用数据库记录和测试回答。
- **验收**：T06–T08 相关用例可重复运行；UI 金额、数据库和手算一致。
- **提交建议**：`docs: explain contract and receivable invariants`。
- **面试主题**：为什么不做通用工作流引擎；多订单如何共享合同额度。

## 第 4 周：回款、冲正与核心闭环

### Day 22 — 回款单与分配模型

- **实现**：回款草稿、唯一凭证号、单订单多期分配；财务权限、客户/订单/期次一致性；应收列表与回款表单。
- **AI 协作**：AI 给分配方案，你用金额字符串独立验证合计，禁用预收/跨订单分配。
- **验收**：销售不能写回款；无效分配、异租户关联、零/负金额被拒绝；草稿不改变余额。
- **提交建议**：`feat: model receipts and installment allocations`；`test: validate receipt ownership and amounts`。
- **面试主题**：回款草稿与记账分离、数据库约束能力边界。

### Day 23 — 记账与幂等

- **实现**：事务锁订单和应收，写记账/分配/余额/审计/outbox；规范化请求摘要；业务唯一凭证号与幂等结果引用。
- **AI 协作**：AI 审查重试路径，你检查重放是否还验证权限、事务失败是否留下成功结果。
- **验收**：T07：同请求多次只有一次记账；换键同凭证不重复；同键不同金额冲突；成功结果一致。
- **提交建议**：`feat: post receipts atomically and idempotently`；`test: prevent duplicate financial effects`。
- **面试主题**：幂等键有效范围、响应丢失后的重试。

### Day 24 — 并发超收与回滚证明

- **实现**：真实 MySQL 并发测试；固定全局锁顺序；在分配后、审计前插入测试故障；有限事务重试与错误分类。
- **AI 协作**：AI 生成压力场景，你检查最终余额、回款数、分配、审计和 outbox，而不是只看状态码。
- **验收**：T09/T10：余额 7000 两笔 5000 仅一笔成功；故障后不残留；重复多轮结果稳定。
- **提交建议**：`test: prove concurrent receipts cannot overcollect`；`fix: enforce receipt lock order and rollback`。
- **面试主题**：死锁、锁超时、业务拒绝与系统错误。

### Day 25 — 冲正与余额对账

- **实现**：全额冲正一次、原因必填、原单保留、补偿分配；结清订单重新打开；实现按记账/冲正流水复算余额的对账脚本。
- **AI 协作**：AI 写恢复逻辑，你检查冲正后取消订单的业务限制与重复冲正竞争。
- **验收**：T11：回款→结清→冲正→未结清一致；并发冲正只有一次；流水复算与余额相同。
- **提交建议**：`feat: reverse posted receipts with audit trail`；`test: reconcile receivables after reversals`。
- **面试主题**：补偿记录与删除原记录、流水与余额的关系。

### Day 26 — 基础报表与口径

- **实现**：销售额、订单批次回款率、本期净回款、逾期、商机赢单率、客户转化率；固定时钟与分母为零处理；以表格为主。
- **AI 协作**：AI 生成 SQL，你用小样本手算并检查多表 JOIN 是否倍增金额。
- **验收**：T15 初版：跨月、冲正、今天到期、零订单、不同数据范围数值正确；指标显示观察日期。
- **提交建议**：`feat: report sales and receivables with explicit definitions`；`test: verify report totals and date boundaries`。
- **面试主题**：聚合粒度、时间区间、金额报表口径。

### Day 27 — 完整闭环 E2E

- **实现**：销售→经理→财务浏览器用例；订单详情汇总合同/应收/净回款/审计；完善错误、重复点击与冲突提示。
- **AI 协作**：AI 运行或补充 E2E，你手动用两个企业、三个岗位复核直接 API 越权。
- **验收**：T01–T11 中核心项通过；重复点击 UI 与接口重放都不会多记钱；录屏能讲通流程。
- **提交建议**：`test: cover customer-to-receipt workflow`；`feat: connect order and collection workspace`。
- **面试主题**：如何分层验证同一业务；按钮禁用为什么不是幂等。

### Day 28 — 轻量复盘与 v0.4 核心交付门槛

- **实现**：整理核心演示、测试证据和已知限制；评估是否进入 SaaS 扩展。核心未通过时使用后续扩展时间修复。
- **AI 协作**：让 AI 模拟面试官，连续追问隔离、审批、并发回款各三层。
- **验收**：能独立解释两企业隔离、一次并发失败、一次事务回滚；当前版本可完整演示。
- **提交建议**：`docs: publish core workflow evidence and limitations`。
- **面试主题**：项目最难的问题与你实际做过的取舍。

## 第 5 周：平台运营、套餐与租户生命周期

### Day 29 — 自助企业注册

- **实现**：注册企业/已有身份加入新企业的受控入口；tenant+owner+基础权益原子创建；请求幂等与频率限制；避免邮箱冒领已有账号。
- **AI 协作**：AI 画新用户与已有用户两条身份链路，你检查绑定需要哪些身份凭证。
- **验收**：双击注册只建一家；中途失败不占孤立租户；限流不依赖前端；公共演示可关闭开放注册。
- **提交建议**：`feat: provision tenant and owner atomically`；`test: secure and deduplicate tenant registration`。
- **面试主题**：身份复用、幂等注册、事务外资源如何处理。

### Day 30 — 套餐与权益快照

- **实现**：免费/专业/企业配置，订阅有效期与权益快照；统一 feature guard；免费保留核心闭环，扩展功能按套餐限制。
- **AI 协作**：AI 生成套餐页面，你检查修改套餐模板会不会意外改变所有历史订阅。
- **验收**：直接调用付费功能 API 同样受限；套餐展示和后端实际权限一致；有版本历史。
- **提交建议**：`feat: model versioned subscription entitlements`；`test: enforce feature access at API boundary`。
- **面试主题**：配置模板、快照、权限与套餐授权的交集。

### Day 31 — 席位配额与并发

- **实现**：活动成员席位计数/对账；新增与启用锁同一配额行；禁用释放；降级超限保留现有成员但禁止新增。
- **AI 协作**：AI 写最后一个席位的并发测试，你确认重新启用不会绕过限制。
- **验收**：T12：并发新增不能超配；禁用/启用计数一致；最后有效管理员不可移除。
- **提交建议**：`feat: enforce transactional seat limits`；`test: race membership changes against quota`。
- **面试主题**：计数器与 COUNT 查询、锁统一资源的意义。

### Day 32 — 到期、禁用与权限变更

- **实现**：active/expired/suspended；过期只读、管理员基础导出、暂停全部企业访问；状态版本；关键业务写事务与生命周期变更按约定锁协调。
- **AI 协作**：AI 搜索只检查菜单的入口，你补 API、附件、报告和将来的任务判断。
- **验收**：T04/T12：旧 token 不绕过禁用；过期后不能审批/回款；并发禁用行为符合文档边界。
- **提交建议**：`feat: enforce tenant lifecycle across business operations`；`test: reject stale access after suspension`。
- **面试主题**：JWT 撤权、生效窗口、缓存失效。

### Day 33 — 平台运营后台

- **实现**：租户搜索、启停、套餐调整、模拟订阅订单；平台关键操作审计；租户数、活跃企业、模拟订阅收入。
- **AI 协作**：AI 生成统计页面，你定义活跃统计窗口及模拟收入状态，避免和企业回款混合。
- **验收**：平台默认无 CRM 内容访问；模拟付款重复请求无双份收入；真实支付入口不存在。
- **提交建议**：`feat: add platform tenant and subscription console`；`test: isolate platform metrics from tenant records`。
- **面试主题**：控制平面与业务数据边界、平台收入与客户回款。

### Day 34 — 套餐生命周期浏览器验证

- **实现**：升级/降级/超限/过期的明确提示；成员列表与剩余席位；补角色修改后刷新菜单和缓存清理。
- **AI 协作**：AI 补 E2E，你从绕过前端、复用旧 tab、直接 URL 三个角度检查。
- **验收**：企业数据不会随降级删除；席位超限有说明；角色与套餐变更后的 API 和页面一致。
- **提交建议**：`feat: explain quota and subscription states in UI`；`test: cover subscription lifecycle flows`。
- **面试主题**：用户体验与后端规则一致性、状态同步。

### Day 35 — 轻量复盘与 v0.5

- **实现**：整理租户生命周期表与席位并发证据，核查新增平台接口是否破坏原有隔离。
- **AI 协作**：AI 做一次权限差异审查，你解释为何平台管理员不是所有企业超级用户。
- **验收**：核心回归仍通过；没有“平台角色直接跳过 tenant 条件”的业务代码。
- **提交建议**：`docs: document SaaS lifecycle and quota guarantees`。
- **面试主题**：多租户 SaaS 与普通后台的根本设计差异。

## 第 6 周：异步提醒、导入导出与可观测性

### Day 36 — 持久任务与 worker

- **实现**：消费已有 outbox；任务领取、lease/claim token、next_run、有限重试和失败状态；worker 优雅退出；数据库保存权威状态。
- **AI 协作**：AI 起草 goroutine worker，你检查 context、ticker 停止、连接回收和并发上限。
- **验收**：进程崩溃后任务可重领；过期 worker 无法覆盖新领取结果；业务事务不等待通知发送。
- **提交建议**：`feat: process durable jobs with leases and retries`；`test: recover interrupted job execution`。
- **面试主题**：goroutine 生命周期、context 取消、至少一次执行。

### Day 37 — 到期与逾期提醒

- **实现**：合同到期/应收逾期扫描、站内通知、业务日去重；可注入时钟；执行前复核租户、接收人、金额；已解决状态。
- **AI 协作**：AI 给日期边界数据，你模拟任务排队后已收清或用户撤权。
- **验收**：T13/T14：双 worker 不重复通知；禁用/到期企业不发；任务执行时已收清不再催收。
- **提交建议**：`feat: notify actionable contract and payment reminders`；`test: deduplicate and revalidate scheduled reminders`。
- **面试主题**：定时扫描与事件驱动、幂等副作用、业务时区。

### Day 38 — 客户导入预览与提交

- **实现**：限定模板的 Excel 导入，大小/解压/行列上限；校验预览与错误行；最多 500 行整批事务，import ID 幂等，tenant 服务端赋值。
- **AI 协作**：AI 写解析器，你检查公式、重复行、非法 owner、跨企业关联和重传文件。
- **验收**：非法行不产生部分成功；重试不重复导入；文件内 tenant 列不能覆盖上下文；不会执行公式。
- **提交建议**：`feat: preview and import customers atomically`；`test: reject unsafe and duplicate import batches`。
- **面试主题**：批处理事务大小、输入边界、可重试导入。

### Day 39 — 私有异步导出

- **实现**：客户/应收导出任务与文件下载；创建/执行/下载三次授权；结果有效期、权限版本；公式样文本作为文本输出。
- **AI 协作**：AI 写任务流程，你测试任务创建后用户被降权或客户被转移。
- **验收**：T05/T14：B 不可拿 A 文件；撤权后不能下载旧结果；到期管理员可做约定的基础导出。
- **提交建议**：`feat: export authorized data through private jobs`；`test: reauthorize export execution and download`。
- **面试主题**：异步任务授权、文件下载凭据与撤权。

### Day 40 — 审计检索与失败任务工作台

- **实现**：按人员/动作/对象/日期查审计；任务列表、失败原因、手动重试；通知中心；审计字段脱敏，禁止普通用户修改审计。
- **AI 协作**：AI 设计筛选组件，你确认输入进入 SQL 是参数化且分页受限。
- **验收**：重复点击重试无重复副作用；已撤权管理员不能重试导出；拒绝日志与成功业务审计都可定位。
- **提交建议**：`feat: expose scoped audit and job history`；`test: authorize manual retry and audit queries`。
- **面试主题**：审计可信边界、重放与重试、日志隐私。

### Day 41 — 日志、指标与运行健康

- **实现**：结构化 requestID/tenant/actor 日志、请求耗时/错误/队列指标、DB 连接池；内部 pprof；区分 liveness/readiness，依赖断开错误清晰。
- **AI 协作**：AI 提议指标，你移除 user/tenant 高基数标签与敏感日志。
- **验收**：从一个失败回款能追到事务错误；worker 失败可定位；metrics/pprof 不向公网开放；日志无 token。
- **提交建议**：`feat: add operational logs and bounded metrics`；`test: handle dependency failures predictably`。
- **面试主题**：可观测性三类信号、连接池、超时与取消。

### Day 42 — 轻量复盘与 v0.6

- **实现**：整理“提交业务后宕机、worker 重领、最终消息一次”的时序图；回顾导入导出越权清单。
- **AI 协作**：AI 故意提出“加 Redis 锁就 exactly-once”，你指出失效与副作用边界。
- **验收**：T13/T14 有可重现测试；任务状态和通知结果一致；核心业务回归仍通过。
- **提交建议**：`docs: explain durable jobs and file authorization`。
- **面试主题**：分布式锁不能替代数据库事务与业务去重。

## 第 7 周：系统性验证与性能证据

### Day 43 — 隔离旁路审查

- **实现**：列完整 API/后台任务/下载入口；逐项检查查询、计数、Preload、原生 SQL、批量更新、缓存键和错误信息；补 T01–T05 遗漏。
- **AI 协作**：AI 按攻击视角提出请求，你确认测试真正使用不同租户且数据库后置断言完整。
- **验收**：所有入口有授权归属和测试链接；不以“用了 Scope”作为未测试豁免。
- **提交建议**：`test: complete tenant isolation regression matrix`；发现缺陷时单独 `fix:`。
- **面试主题**：对象级越权、批量接口、关联与聚合旁路。

### Day 44 — 并发与故障矩阵

- **实现**：重复/并发审批、订单额度、记账、冲正、席位、任务重领；注入超时和 DB 错误；业务不变量对账脚本进入 CI。
- **AI 协作**：AI 调整测试屏障与并发起跑，你排查偶发失败而非加 sleep 掩盖。
- **验收**：T06–T14 重复跑稳定；余额/流水/事件一致；死锁被有限重试或明确返回，不无限循环。
- **提交建议**：`test: exercise concurrency and rollback failure matrix`；`test: verify financial invariants after stress`。
- **面试主题**：并发测试确定性、故障注入、事务重试边界。

### Day 45 — Go 基础质量与运行时诊断

- **实现**：go vet、业务包测试、race 检查；检查 goroutine 泄漏、context 传递、错误包装和 HTTP/DB 超时；跑一个 pprof 样例。
- **AI 协作**：AI 给出问题候选，你用堆栈/测试证据确认，不盲目引入抽象。
- **验收**：受支持环境中 race 检查通过；关闭 worker 后活动协程回落；没有请求取消后无限后台任务。
- **提交建议**：`fix: bound worker and request lifecycles`；`test: verify cancellation and concurrent memory access`。
- **面试主题**：channel/mutex、内存竞争与数据库竞争、pprof。

### Day 46 — 数据集与压测基线

- **实现**：固定 seed、10 租户分档数据；客户分页/应收/报表/混合读写脚本；登记硬件、版本、运行参数与原始结果。
- **AI 协作**：AI 帮写负载脚本，你核对实际请求率、业务成功率及数据分布。
- **验收**：压测可从空测试库重现；预期 409 与 5xx 分列；不得写入演示/真实库；性能数字尚未优化也照实记录。
- **提交建议**：`perf: add reproducible multi-tenant workloads`；`docs: record initial performance baseline`。
- **面试主题**：QPS、延迟分位、并发与吞吐、压测有效性。

### Day 47 — SQL 和分页优化

- **实现**：选择实际最慢查询做 EXPLAIN/EXPLAIN ANALYZE，检查复合索引、N+1、聚合粒度；深分页按需要改游标；相同负载复测。
- **AI 协作**：AI 给候选索引，你用执行计划与实测决定，不把所有筛选列都建索引。
- **验收**：记录扫描行数/延迟/写入代价变化；优化后权限和金额测试仍通过；无收益也保留结论。
- **提交建议**：`perf: optimize measured receivable query hotspot`；`docs: compare query plans and benchmark results`。
- **面试主题**：覆盖索引、最左前缀、offset 与 keyset、写放大。

### Day 48 — 前端体验与回归

- **实现**：表格分页、请求竞态、组件复用、表单错误定位、权限状态、长金额显示、基本键盘操作；优化一个测到的页面问题。
- **AI 协作**：AI 审查 Vue watch/computed 与请求链，你避免无证据的过度缓存。
- **验收**：筛选快速切换不串数据；切租户不残留；核心 E2E 与组件测试通过；构建无新增类型错误。
- **提交建议**：`fix: stabilize business forms and async table state`；`test: cover role and tenant UI transitions`。
- **面试主题**：Vue 响应式、组件边界、异步竞态和错误体验。

### Day 49 — 轻量复盘与 v0.7

- **实现**：汇总 T01–T16 状态与性能报告草稿，修复阻塞发布的问题；只记录实际运行过的命令和结果。
- **AI 协作**：AI 检查报告是否把目标写成实测、单机写成分布式高可用。
- **验收**：每个技术亮点都能指向代码、测试或测量；未覆盖项明确列出。
- **提交建议**：`docs: publish verification and performance evidence`。
- **面试主题**：怎样评价一个作品的工程可信度。

## 第 8 周：部署、展示与面试材料

### Day 50 — 完整 Compose 部署

- **实现**：应用/worker/MySQL/Redis/Nginx；多阶段构建、固定版本、私有卷、健康检查；迁移与 seed 为显式命令；关闭开发管理入口。
- **AI 协作**：AI 起草容器配置，你检查秘密、监听端口、用户权限和数据持久化。
- **验收**：干净目录按文档构建运行；容器重启数据不丢；不依赖本机绝对路径；普通启动不清库。
- **提交建议**：`build: ship reproducible Compose deployment`；`docs: document configuration and startup`。
- **面试主题**：容器镜像与卷、反向代理、配置与秘密。

### Day 51 — CI 与升级、恢复演练

- **实现**：PR 跑 Go/Vue/真实 MySQL 集成与关键 E2E；构建镜像；在测试环境演练旧版本迁移、备份恢复，记录数据库兼容边界。
- **AI 协作**：AI 给演练清单，你实际执行并确认恢复后金额对账与租户测试。
- **验收**：CI 从干净环境通过；恢复后数据可读且对账一致；代码回滚不被误当成数据库自动回滚。
- **提交建议**：`ci: validate release and database integration`；`docs: record migration and restore drill`。
- **面试主题**：持续集成、向后兼容迁移、备份与恢复验证。

### Day 52 — 演示数据与访问方式

- **实现**：固定 A/B 企业、销售/经理/财务合成账号；一键 seed、演示脚本；可选公网 HTTPS 演示，平台危险写操作仅本地或受限沙箱。
- **AI 协作**：AI 生成逼真但虚构的业务数据，你检查数字可手算和不存在真实个人信息。
- **验收**：访客可理解账号权限；公网部署若未完成提供录屏+本地复现；不把本地地址写成在线站点。
- **提交建议**：`feat: provide reproducible isolated demo data`；`docs: explain demo roles and access`。
- **面试主题**：演示环境隔离、多访客污染与受限权限。

### Day 53 — README、ER 与 API 交付

- **实现**：首屏业务截图/演示入口/启动命令；ER 与真实迁移核对；API 参数、金额、幂等、状态与错误；UPSTREAM 和个人贡献表。
- **AI 协作**：AI 润色，你逐条检查链接/命令/截图与当前实现一致。
- **验收**：不看聊天记录也能理解项目、启动和验证；文档不宣传未实现功能。
- **提交建议**：`docs: publish architecture ER and API guide`；`docs: distinguish upstream and project contributions`。
- **面试主题**：技术沟通、设计决策记录、开源复用归属。

### Day 54 — 录屏与项目讲述

- **实现**：录 5–8 分钟核心演示、补 1 分钟平台套餐；展示越权失败、并发回款、审计与 CI；准备 90 秒/5 分钟两种介绍。
- **AI 协作**：AI 提炼讲稿，你删除“高可用、海量高并发、企业生产级”等无证据表述。
- **验收**：视频从真实程序操作录制；关键结果能与测试对应；README 视频入口可访问。
- **提交建议**：`docs: add recorded walkthrough and presentation notes`。
- **面试主题**：用问题、方案、验证、限制讲项目。

### Day 55 — 面试与现场修改训练

- **实现**：完成手册 60 题中的薄弱项；独立写金额解析、Tenant Scope 测试或回款事务伪代码；做一个真实小需求并完整验证。
- **AI 协作**：让 AI 只当面试官和审查者；先自己写，再让它找漏洞。
- **验收**：能独立定位一次越权/重复记账缺陷；解释 Go 基础和 SQL，不只是复述 README。
- **提交建议**：`docs: record interview gaps and tradeoffs`；如有真实改动，单独 `feat:`/`test:`。
- **面试主题**：基础题、项目追问、临场编码与故障排查。

### Day 56 — 最终验收与 v1.0

- **实现**：从全新 clone 验证启动、迁移、seed、测试、闭环、隔离；发布 v1.0 Release，列功能、证据、已知限制和下一阶段范围。
- **AI 协作**：AI 按发布清单审查，你亲自签收所有关键证据；缺项不伪装完成。
- **验收**：T01–T16 结果清楚、README 无坏链、CI 通过、演示可复现；根据真实成果填写简历。
- **提交建议**：`docs: publish v1.0 release evidence and limitations`；仅修复发现的发布问题。
- **面试主题**：如果再给一个月，你会先改什么，为什么。

## 卡住时怎么调整

| 情况 | 调整方式 |
|---|---|
| Go 语法和 GVA 源码都读不懂 | 加基础周，先独立完成 handler→service→GORM→httptest 小练习 |
| Day 14 仍有租户隔离漏洞 | 推迟报表美化和套餐统计，优先补可信上下文与关联约束 |
| Day 28 金额/事务不可靠 | 用第 5 周修复核心；套餐仅保留固定计划与到期控制 |
| 异步/Excel 扩展超时 | 提醒只做站内；导入只做客户模板；导出保留基础场景 |
| 部署条件不足 | 完成可复现 Compose 与录屏；公开写明尚无在线站点 |
| 提交数少于预期 | 正常；补真实功能和验证，不制造历史、空提交或无意义格式改动 |

每天结束记录：Issue、实际变更、测试命令/结果、发现问题、你亲自做的判断、下一步。记录能帮助面试复盘；不要把 AI 生成的摘要当成未经检查的完成证明。
