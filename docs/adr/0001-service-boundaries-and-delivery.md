# ADR 0001：核心事务边界与可靠事件传递

日期：2026-10-24；2026-10-25 更新。状态：保留为后续设计，当前仅交付 Day 1；S1 代码已本地归档，服务化实施暂停。

用户将范围扩展至套餐、工作流、异步任务、微服务、Kafka 和 Kubernetes。原 GVA 仅完成 Day 1 基线，当前最重要的依赖是可信租户、金额事务与失败恢复的可证明性。

决策：保留 core-api 的合同、订单、应收、回款以及审批决策在一个数据库事务边界内；先独立 automation-service，并用 outbox→Kafka→inbox 传递业务事件。两服务不共享写表，不靠分布式事务承诺 MySQL 与 Kafka 同时提交。订阅与客户回款使用不同模型和统计口径。

S1 是本地操作员使用的 `subscription-lab` 与独立进程 `subscription-worker`，两个专用 schema、显式 SQL 迁移、无公开 API。其 lab_tenants 不是正式成员/租户授权模型。worker 暂时同时连接两个库，通过本地回调交付；Kafka 阶段拆成受限的 core relay 与独立 automation consumer，并加入服务认证、topic ACL 与信封版本协议。这个临时桥接不能直接部署为公开服务。

S1 不引入 GVA 的全局 DB，也不注册 ServiceGroupApp/公共 Router；通过模块 enter.go 注入数据库，避免未鉴权模拟命令变成管理接口。正式业务 API 阶段恢复项目 Router→API→Service→Model 与统一响应要求。

S1 迁移每库独立串行、记录 checksum；MySQL DDL 会隐式提交，脚本按可重试的 CREATE TABLE 设计。checksum 能发现已登记脚本被改，不能证明手工修改后的表结构没有漂移。跨库迁移失败可重复运行，两个版本都验证后才启动 worker，不宣称跨库 DDL 原子性。

消费者 inbox 与开通待办同事务；确认丢失允许重投。同 event_id 内容改变时拒绝；订单等有序业务将增加 aggregate_version 与版本缺口处理。外部邮件/第三方接口不能靠本地 inbox 自动获得唯一副作用，要在后续另定义幂等契约。

取舍：减少核心资金不变量的分布式复杂度，仍可练习独立异步服务、消息和部署。代价是核心服务规模较大、事件与跨服务读取需要版本管理。继续拆分核心必须以实际需求和 Saga/补偿/对账测试为前置条件。

依据：[Kafka 交付语义](https://kafka.apache.org/41/design/design/)、[Kubernetes 探针](https://kubernetes.io/docs/concepts/workloads/pods/probes/)。这些资料支持消息/运行机制，不代表本项目已经通过对应故障测试。
