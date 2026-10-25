# GVA 来源与修改记录

本项目基于 https://github.com/flipped-aurora/gin-vue-admin 。

| 项目 | 固定值 |
|---|---|
| Tag | `v2.9.2-stable` |
| Commit | `a6882210a80bb27e3aa5dff0b4c21aa4afe8988a` |
| 获取日期 | 2026-10-24（Asia/Shanghai） |
| 远端 | `upstream` 保留官方来源；项目 `origin` 为 https://github.com/JoeyXXia/goCRM.git |
| Go module | 保留 `github.com/flipped-aurora/gin-vue-admin/server`，避免无关导入重写 |
| Go 声明 | 上游 `go 1.24.0` / `toolchain go1.24.2`，保留原值 |
| 本机运行工具 | Go 1.26.5 windows/amd64、Node 22.14.0、npm 10.9.0、MySQL 8.0.44 |
| 锁文件 | 原始 `server/go.sum`；本项目新增 `web/package-lock.json`，使用 npm ci 安装 |

以此 SHA 的 [LICENSE](./LICENSE) 为准：实际文件为 Business Source License 1.1，Change License 为 Apache 2.0；不能把 Change License 或上游 README 的 Apache 描述当作当前许可。复制的规划文档是执行前设计，许可描述由本记录纠正。保留原始 LICENSE、README、作者注释、品牌、构建横幅及适用声明；没有发现独立 NOTICE 文件。今天只做本地开发验证，不发布部署。

## 2026-10-24 / Day 1 改动

- 新增 DealFlow 项目说明、业务范围、源码链路、启动指引、环境模板、工具版本记录和每日进度。
- 根 README 顶部新增项目入口和日期声明，保留全部上游正文；详细项目入口见 [DEALFLOW.md](./DEALFLOW.md)。
- 根 `.gitignore` 新增本地秘密与运行目录排除，`web/.gitignore` 允许提交 npm 锁文件；修改处注明日期。
- 新增前端锁文件，保留上游 Go module、go.mod/go.sum 和应用组织方式。
- 新增本地配置准备脚本，不更改原始应用代码，不实现租户或回款功能。

登录、JWT、Casbin 接入、动态菜单、后台布局、Swagger、生成器和示例客户属于上游。TenantContext、tenant domain、数据范围、合同版本、回款事务、outbox 和隔离测试仍是计划新增能力。

验证归属：`git diff a688221 -- server web`；上游文件修改列表：`git diff --name-status a688221`。

## 2026-10-24 / S1 扩展

新增 `server/model/subscriptionlab`、`server/service/subscriptionlab`、两个独立 CLI 入口和真实 MySQL 集成测试；更新扩展计划、服务边界 ADR、运行与进度文档。此阶段采用显式 SQL 迁移和注入的独立实验库，未修改 GVA 的登录、公共 Router、业务模型或依赖锁文件，也未声称已实现正式租户、Kafka 或 Kubernetes。

2026-10-25 范围收回：按用户最新“只做到 Day 1”的要求，上述 S1 代码、脚本和运行说明已移至 `.local/deferred-stage1/2026-10-25/` 并校验归档哈希，当前不再位于应用源码中。扩展计划保留为后续参考；Day 1 的固定版本与原始应用基线不变。
