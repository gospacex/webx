# webx-middleware Specification

## ADDED Requirements

### Requirement: 中间件链默认装配与顺序
`webx.New` SHALL 自动装配默认中间件链，顺序固定为：
`recovery → otel(root span + trace-id) → logx 日志 → ratelimit → cors → timeout → 业务 handler`。
`webx.WithMiddleware(mws ...app.HandlerFunc)` 追加的中间件 SHALL 位于默认链之后、业务 handler 之前。

#### Scenario: 默认链顺序
- **WHEN** `webx.New()` 后发起正常请求
- **THEN** 响应头携带 trace-id（`X-Request-Id`）；panic 请求被 recovery 捕获返回 500 而不使进程崩溃

#### Scenario: 追加业务中间件
- **WHEN** `webx.New(webx.WithMiddleware(authCheck))` 后无凭证请求
- **THEN** 业务中间件在默认链之后、handler 之前执行，返回 401 envelope

### Requirement: recovery 中间件
recovery SHALL 捕获 handler 链中的 panic，记录 error 级日志（带 context、request_id、堆栈），并渲染 envelope 响应 code=5001、HTTP 500。

#### Scenario: panic 被捕获
- **WHEN** handler 中 `panic("boom")`
- **THEN** 服务不崩溃；响应为 500 且 body code=5001；日志含 panic 值与堆栈

### Requirement: timeout 中间件
timeout 中间件 SHALL 按配置超时（默认 5s）对请求 ctx 施加截止时间；超时 SHALL 渲染 envelope code=5002（HTTP 状态码 504）。

#### Scenario: 慢请求超时
- **WHEN** handler 耗时超过配置 timeout
- **THEN** 客户端收到 504 响应，code=5002，且后续日志 ctx 已取消

### Requirement: ratelimit 中间件
ratelimit SHALL 基于令牌桶（按 remote IP 维度）限制请求速率，超限 SHALL 返回 429 且 envelope code=4290。

#### Scenario: 超限请求
- **WHEN** 同 IP 在窗口内超过限流阈值
- **THEN** 响应 429，body code=4290；未超限时不干预

### Requirement: cors 中间件
cors SHALL 按配置允许 origins/headers 放行跨域预检；未配置的 origins SHALL 拒绝预检。

#### Scenario: 预检放行与拒绝
- **WHEN** OPTIONS 预检携带已配置 origin
- **THEN** 响应包含 `Access-Control-Allow-Origin`；未配置 origin 时不放行

### Requirement: otel 与 trace-id 注入
otel 中间件 SHALL 为每个请求创建 root span（span 名 `<METHOD> <path>`），并将 trace-id 写入请求 ctx 与响应头 `X-Request-Id`；logx 日志经 context 传播 SHALL 自动携带 trace_id/span_id。

#### Scenario: trace-id 贯穿
- **WHEN** 客户端带 `X-Request-Id` 请求头发起调用（或无头由服务端生成）
- **THEN** 响应头 `X-Request-Id` 与日志中 trace_id 可对应；无请求头时服务端生成 uuid 作为 request_id

### Requirement: logx 日志中间件
logx 中间件 SHALL 在请求结束时输出一条带 context 的 access 日志（method/path/remote_addr/status/duration，caller 归因到调用点）。

#### Scenario: access 日志
- **WHEN** 一次请求完成
- **THEN** 日志条目含 method、path、status、duration 字段，且 caller 指向业务调用处而非框架层
