# webx-observability Specification

## ADDED Requirements

### Requirement: health 端点
`webx.New` SHALL 自动挂载 health 端点（配置 `webx.health.prefix`，默认 `/healthz`）：`/healthz/live`（存活，恒 200）与 `/healthz/ready`（就绪，依赖可选 `ReadyCheck func() error` 选项，未提供时恒就绪）。

#### Scenario: 存活探针
- **WHEN** GET `/healthz/live`
- **THEN** HTTP 200，body `{"status":"alive"}`

#### Scenario: 就绪探针失败
- **WHEN** 提供 `webx.WithReadyCheck(func() error { return errDB })` 后 GET `/healthz/ready`
- **THEN** HTTP 503，body 含错误文案

### Requirement: OpenTelemetry 集成
系统 SHALL 提供 `webx.WithOTLP(endpoint string)` 与 `webx.WithMetrics()` 选项：初始化全局 TracerProvider（OTLP gRPC 导出）与 Meter（指标：请求计数、耗时直方图、按 code 维度），`App.Shutdown` SHALL 同时 flush trace 与 metrics。

#### Scenario: span 导出
- **WHEN** 启用 OTLP exporter 后发起一次请求
- **THEN** 导出一个 root span（名 `GET /path`）含 http.method/http.route 属性，span 结束事件随请求完成

#### Scenario: 指标上报
- **WHEN** 处理 100 个请求（含 10 个 code!=0）
- **THEN** metrics 含请求总数 100、按 code 维度错误计数 10

### Requirement: TLS / HTTPS
`webx.WithTLS(certFile, keyFile)` SHALL 启用 HTTPS 并开启 H2；证书加载失败 SHALL 在 `New` 阶段 fail fast 返回错误（不在运行期静默降级为 HTTP）。

#### Scenario: HTTPS 正常服务
- **WHEN** 配置自签证书后启动，客户端带 InsecureSkipVerify 访问
- **THEN** 连接协议为 h2，请求正常返回 envelope 响应

#### Scenario: 证书缺失 fail fast
- **WHEN** `webx.New(webx.WithTLS("/nonexistent.crt", "/nonexistent.key"))`
- **THEN** 返回明确错误（含文件路径），不 panic
