# webx-examples Specification

## ADDED Requirements

### Requirement: 分层示例工程结构
`examples/` SHALL 提供企业级分层 demo 工程，目录结构固定为：
`cmd/main.go`（server 层一行启动）、`app/handler/`（hz 生成区，薄 handler）、`app/routes.go`（注册源唯一）、`internal/middleware/`（业务中间件，如鉴权）、`internal/logic/`（薄适配：参数转换 + 调 service）、`internal/service/`（业务核心）、`internal/model/`（entity + Repository 接口 + 内存 demo 实现）、`internal/config/`（业务侧配置结构，configx 绑定）。
`app/` 目录 SHALL 仅存放 hz 生成物，手写代码不得放入其中。

#### Scenario: 一行启动
- **WHEN** 执行 `go run ./cmd` 并访问 demo 接口
- **THEN** 经 configx 加载业务配置、logx 初始化日志、webx bootstrap 完成装配，demo 接口返回 envelope 成功响应

#### Scenario: hz 生成区约定
- **WHEN** 重新运行 hz 生成器
- **THEN** 仅影响 `app/` 目录；`internal/` 下手写代码（含业务中间件）不被生成器触碰

### Requirement: 各层职责与调用链归因
示例各层 SHALL 按 中间件 → handler → logic → service → model 单向调用；每层日志经 logx context 方法输出，caller 归因 SHALL 指向该层真实调用点（非框架层）。

#### Scenario: 三层日志 caller 归因
- **WHEN** 一次 demo 请求触发 handler/logic/service/model 四层日志
- **THEN** 四条日志的 caller 字段分别指向各层自身文件行号，且均携带同一 trace_id

### Requirement: 端到端调用链贯通
SHALL 存在端到端测试用例：请求携带 `X-Request-Id` → 服务处理 → 断言响应 `request_id` 与请求头一致，且日志可检索到同 id 的 handler、service、model 三层条目。

#### Scenario: 调用链贯穿断言
- **WHEN** 运行端到端用例（TestApp 随机端口 + 真 HTTP 请求）
- **THEN** 响应 request_id == 请求 trace-id；日志捕获 buffer 中三层条目 trace_id 全部相同

### Requirement: 分层测试覆盖
examples SHALL 包含：handler/logic/service/model 各层单测（model 以内存实现 + 接口注入）；每个 webx 中间件（recovery/timeout/ratelimit/auth/otel/logx）至少一个功能用例；webx 核心包（webx 根 + middleware + envelope）覆盖率 SHALL ≥ 80%。

#### Scenario: 覆盖率达标
- **WHEN** 执行 `go test -cover ./...`
- **THEN** webx 核心三包行覆盖率均 ≥ 80%，examples 全层测试通过

### Requirement: 调用链开关（分层路由入链）
`examples/` SHALL 提供调用链开关（业务配置 `demo.trace.enabled`，默认 false；导出器 `demo.trace.exporter`，默认 stdout，经 configx 绑定至 `internal/config`）：
- **开启时**：cmd 层 SHALL 以 `OTel.Enabled=true` 构造全局 logx logger（`logx.Config.OTel`），并经 logx 装配 OTel TracerProvider（`logx/zap.SetupTracerProvider`，导出器取配置值）；**分层路由入链**——路由入口（`routes.go` 注册的入口中间件）SHALL 开 root span（名 `<METHOD> <path>`），handler/logic/service/model 各层 SHALL 各自开子 span（`tracer.Start(ctx, "<层>.<操作>")`）写入 ctx；各层日志 SHALL 经 logx 的 context 方法（`InfoContext` 等）输出——logger `OTel.Enabled=true` 时，logx 自动将日志事件挂接为 ctx 中活动 span 的 span event 并附加 trace_id/span_id 字段，即完成"logx 入链"。
- **关闭时**（默认）：不初始化 TracerProvider、各层不开 span；logger `OTel.Enabled=false`，各层经 logx 普通 context 日志输出（仅 request_id 可检索），服务行为不变。

#### Scenario: 开关开启各层入链
- **WHEN** `demo.trace.enabled=true` 启动 demo 并请求一次 demo 接口
- **THEN** 导出器收到一棵 span 树（root + handler/logic/service/model 子 span）；四层 logx 日志条目均带同一 trace_id，且日志事件出现在对应层 span 的 events 中

#### Scenario: 开关关闭（默认）
- **WHEN** 以默认配置 `demo.trace.enabled=false` 启动 demo
- **THEN** 未初始化 TracerProvider、无 span；各层经 logx 普通日志正常输出；请求正常返回 envelope
