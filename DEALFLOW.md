# DealFlow CRM · 企销云

基于 GVA 的多租户销售与回款学习项目。按用户 2026-10-25 的最新要求，目前只做到 Day 1。正式租户授权、交易闭环、工作流、Kafka 和 Kubernetes 保留为后续规划，暂不实施。

项目入口：[范围与故事](docs/scope.md) · [启动](docs/day1-setup.md) · [上游归属](UPSTREAM.md) · [身份调用链](docs/architecture-day1.md) · [今日进度](docs/progress/2026-10-24-day1.md) · [56 天计划](docs/planning/DealFlow-CRM-56天每日计划.md)。

后续参考：[扩展整体计划](docs/planning/DealFlow-CRM-扩展实施计划.md) · [服务边界设计](docs/adr/0001-service-boundaries-and-delivery.md) · [当前范围与归档记录](docs/progress/2026-10-25-scope-reset.md)。

源码沿用 `server/`（Go/Gin/GORM）与 `web/`（Vue 3/Vite/Pinia）；逐步独立异步服务，核心保留本地事务。[README](README.md) 保留全部上游正文并在顶部新增项目入口，[LICENSE](LICENSE) 原样保留。许可和固定版本详见 UPSTREAM，不以规划或上游宣传文字替代实际许可证。

快速启动：执行 `./scripts/prepare-local.ps1`；在 `server/` 执行 `go mod download`、`go run . -c config.local.yaml`；另一个终端在 `web/` 执行 `npm ci`、`npm exec vite -- --host 127.0.0.1 --mode development`。访问 http://127.0.0.1:8080 ，在初始化页面配置专用 MySQL 空库并设置管理员密码。完整前提与验收见启动文档。

超出 Day 1 的 S1 实验代码、脚本与运行记录已移入忽略的本地归档目录，当前 server/web 应用源码保持上游基线。上游示例客户不代表本项目已经完成多租户 CRM。
