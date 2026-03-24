[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/rc-yile-dispatch/release.yml?branch=main&label=BUILD)](https://github.com/yylego/rc-yile-dispatch/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/rc-yile-dispatch)](https://pkg.go.dev/github.com/yylego/rc-yile-dispatch)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/rc-yile-dispatch/main.svg)](https://coveralls.io/github/yylego/rc-yile-dispatch?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.26+-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/badge/release-active-blue.svg)](https://github.com/yylego/rc-yile-dispatch)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/rc-yile-dispatch)](https://goreportcard.com/report/github.com/yylego/rc-yile-dispatch)

# rc-yile-dispatch

API 通知分发服务 — 接收业务系统提交的 HTTP 通知任务，以至少一次（at-least-once）语义可靠地投递到外部 API。

---

<!-- TEMPLATE (ZH) BEGIN: LANGUAGE NAVIGATION -->

## 英文文档

[ENGLISH README](README.md)

<!-- TEMPLATE (ZH) END: LANGUAGE NAVIGATION -->

## 问题理解

企业内部多个业务系统在关键事件发生时，需要调用外部供应商的 HTTP API 进行通知。不同供应商的 URL、Header、Body 格式各不相同。业务系统不关心外部 API 的返回值，只需确保通知能稳定送达。

核心挑战**不是 HTTP 调用本身**，而是**可靠性**：外部 API 可能宕机、超时、返回错误。我们需要一个服务来接收任务、持久化存储、并持续重试直到成功。

## 架构

```
业务系统                          本服务                          外部 API
  |                                |                                |
  |  POST /api/dispatch            |                                |
  |  {url, headers, body}          |                                |
  |------------------------------->|                                |
  |                                |  1. 持久化到 SQLite             |
  |  {"id":1, "status":"pending"}  |                                |
  |<-------------------------------|                                |
  |                                |  2. 后台 Worker 轮询            |
  |                                |     待投递任务                  |
  |                                |                                |
  |                                |  3. 发送 HTTP 请求 ----------->|
  |                                |                                |
  |                                |  4a. 2xx → 标记成功            |
  |                                |  4b. 失败 → 指数退避           |
  |                                |       重试直到最大次数          |
  |                                |  4c. 超过最大重试 → 死信       |
```

## 设计决策

### 1. 系统边界

**本服务负责的：**

- 通过 HTTP API 接收通知任务
- 持久化任务防止数据丢失
- 向目标 URL 发送 HTTP 请求
- 自动重试（指数退避）
- 超过最大重试次数进入死信状态

**本服务不负责的：**

- 解析或校验外部 API 的响应体 — 只检查状态码
- 在系统间转换请求格式 — 业务系统必须提供完整的 URL/Header/Body
- 按供应商限流 — MVP 范围外，v2 再加
- 管理外部 API 的认证信息 — 业务系统自己在 Header 里带 token

### 2. 投递语义：至少一次（At-Least-Once）

通知**可能被发送多次**（例如：外部 API 收到了请求但我们的服务在标记成功之前崩溃了）。这是可接受的，因为：

- 需求明确说业务系统不关心返回值
- 大多数通知 API 是幂等的或容忍重复
- 恰好一次（exactly-once）需要分布式事务，在 MVP 阶段是严重的过度设计

### 3. 失败处理

**指数退避重试：**

- 第1次失败 → 等2秒 → 第2次
- 第2次失败 → 等4秒 → 第3次
- 第3次失败 → 等8秒 → 第4次
- 最多重试可配置次数（默认5次）

**死信状态：** 用尽所有重试次数后，任务进入 `deadline` 状态，意味着外部系统长时间不可用，需要人工介入。生产环境中这会触发告警。

**什么算失败：**

- 网络错误、超时、连接拒绝
- HTTP 响应状态码非 2xx
- 请求构建失败（URL 格式错误等）

### 4. 技术选型与取舍

| 选择         | 理由                           | 未采用的替代方案                             |
| ------------ | ------------------------------ | -------------------------------------------- |
| SQLite       | 单二进制部署，零配置，MVP 足够 | PostgreSQL（生产环境）、Redis（易丢失）      |
| 数据库当队列 | 简单可靠，不需要额外基础设施   | RabbitMQ/Kafka（MVP 阶段过度设计）           |
| 轮询 Worker  | 实现简单，负载可预测           | 每任务一个协程（资源密集）、事件驱动（复杂） |
| `net/http`   | 标准库，零框架开销             | Gin/Echo/Kratos（不必要的抽象层）            |

**为什么不用消息队列（Kafka/RabbitMQ）？**

- 增加运维复杂度（部署、监控、配置）
- MVP 流量级别下，数据库轮询完全够用
- "队列"语义通过任务状态 + NextRunAt 在数据库中实现
- 如果流量增长100倍，迁移到专用队列很直接 — Store 接口抽象了持久化层

**为什么不用 Web 框架？**

- 3 个接口不值得引入框架
- `net/http` + `ServeMux` 足够，零依赖
- 体现决策是基于实际需求，而非习惯

### 5. 未来演进

如果成为有显著流量的生产服务：

1. **阶段1：水平扩展** — SQLite 换 PostgreSQL，多 Worker 实例配合行级锁（`SELECT ... FOR UPDATE SKIP LOCKED`）
2. **阶段2：消息队列** — 数据库轮询换 RabbitMQ/Kafka，提升吞吐和降低延迟
3. **阶段3：供应商配置** — 添加供应商注册表，配置限流、熔断器、自定义超时
4. **阶段4：可观测性** — Prometheus 指标（投递延迟、成功率、队列深度），分布式追踪

## API 接口

### POST /api/dispatch — 提交通知任务

```json
{
  "method": "POST",
  "targetUrl": "https://external-api.example.com/webhook",
  "headers": {
    "Authorization": "Bearer xxx",
    "Content-Type": "application/json"
  },
  "body": "{\"event\":\"user_registered\",\"userId\":123}",
  "maxRetries": 5,
  "callback": "ad-system-registration"
}
```

响应：

```json
{ "id": 1, "status": "pending", "message": "task accepted" }
```

### GET /api/task?id=1 — 查询任务状态

### GET /api/tasks?status=pending&page=1&pageSize=20 — 列表查询

### GET /health — 健康检查

## 快速启动

```bash
cd cmd
go run main.go
# 服务启动在 :8088

# 提交测试任务
curl -X POST http://localhost:8088/api/dispatch \
  -H 'Content-Type: application/json' \
  -d '{"method":"POST","targetUrl":"https://httpbin.org/post","body":"{\"test\":true}"}'

# 查看任务状态
curl http://localhost:8088/api/task?id=1
```

## 项目结构

```
rc-yile-dispatch/
├── cmd/main.go                  # 入口：HTTP 服务 + 分发器启动
├── internal/
│   ├── model/task.go            # 任务实体（数据库表结构）
│   ├── store/store.go           # 数据库操作（增删改查）
│   ├── handler/submit.go        # HTTP 处理器（提交/查询任务）
│   └── worker/dispatch.go       # 后台 Worker（轮询+投递+重试）
├── go.mod
└── README.md
```

## AI 使用说明

### AI 提供帮助的地方

- 搭建初始项目结构和样板代码
- 编写 store.MarkFailed 中的指数退避逻辑
- 生成带输入校验的 HTTP 处理器

### 未采纳的 AI 建议

- AI 建议使用 Redis 作为任务队列 — 拒绝，因为增加了基础设施依赖，在 MVP 规模下没有明显收益。SQLite 持久化更简单且更可靠（重启不丢数据）
- AI 建议添加基于 YAML 配置文件的按供应商配置 — 拒绝，MVP 阶段过度设计。业务系统已经知道目标 URL/Header，不需要维护供应商注册表
- AI 建议实现熔断器模式（Circuit Breaker）— 拒绝，MVP 阶段指数退避重试已经能达到类似的保护效果，不需要状态机管理的复杂度

### 我自己做出的关键决策

- **数据库当队列模式**：基于构建消息分发系统的经验。中低流量下，有适当索引（status + next_run_at）的数据库性能足够，且消除了基础设施依赖
- **至少一次语义**：选择不尝试恰好一次，因为业务需求明确说"不关心返回值" — 重复通知无害，丢失通知有害
- **单进程架构**：一个进程 + 内嵌 SQLite 意味着部署就是复制一个二进制文件。不需要 Docker Compose、服务网格、配置管理。当目标是"能用的第一版"时，这很重要
- **不做供应商抽象层**：业务系统提供完整请求（URL + Header + Body）。添加供应商注册表意味着假设我们提前知道所有供应商，这与"不同供应商有不同格式"的需求矛盾 — 让调用方决定格式

---

<!-- TEMPLATE (ZH) BEGIN: STANDARD PROJECT FOOTER -->

## 📄 许可证类型

MIT 许可证 - 详见 [LICENSE](LICENSE)。

---

## 💬 联系与反馈

非常欢迎贡献代码！报告 BUG、建议功能、贡献代码：

- 🐛 **问题报告？** 在 GitHub 上提交问题并附上重现步骤
- 💡 **新颖思路？** 创建 issue 讨论
- 📖 **文档疑惑？** 报告问题，帮助我们完善文档
- 🚀 **需要功能？** 分享使用场景，帮助理解需求
- ⚡ **性能瓶颈？** 报告慢操作，协助解决性能问题
- 🔧 **配置困扰？** 询问复杂设置的相关问题
- 📢 **关注进展？** 关注仓库以获取新版本和功能
- 🌟 **成功案例？** 分享这个包如何改善工作流程
- 💬 **反馈意见？** 欢迎提出建议和意见

---

## 🔧 代码贡献

新代码贡献，请遵循此流程：

1. **Fork**：在 GitHub 上 Fork 仓库（使用网页界面）
2. **克隆**：克隆 Fork 的项目（`git clone https://github.com/yourname/repo-name.git`）
3. **导航**：进入克隆的项目（`cd repo-name`）
4. **分支**：创建功能分支（`git checkout -b feature/xxx`）
5. **编码**：实现您的更改并编写全面的测试
6. **测试**：（Golang 项目）确保测试通过（`go test ./...`）并遵循 Go 代码风格约定
7. **文档**：面向用户的更改需要更新文档
8. **暂存**：暂存更改（`git add .`）
9. **提交**：提交更改（`git commit -m "Add feature xxx"`）确保向后兼容的代码
10. **推送**：推送到分支（`git push origin feature/xxx`）
11. **PR**：在 GitHub 上打开 Merge Request（在 GitHub 网页上）并提供详细描述

请确保测试通过并包含相关的文档更新。

---

## 🌟 项目支持

非常欢迎通过提交 Merge Request 和报告问题来贡献此项目。

**项目支持：**

- ⭐ **给予星标**如果项目对您有帮助
- 🤝 **分享项目**给团队成员和（golang）编程朋友
- 📝 **撰写博客**关于开发工具和工作流程 - 我们提供写作支持
- 🌟 **加入生态** - 致力于支持开源和（golang）开发场景

**祝你用这个包编程愉快！** 🎉🎉🎉

<!-- TEMPLATE (ZH) END: STANDARD PROJECT FOOTER -->
