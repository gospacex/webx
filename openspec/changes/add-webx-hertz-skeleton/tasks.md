# Tasks: add-webx-hertz-skeleton

## 1. 模块脚手架
- [ ] 1.1 初始化 `hubx/webx` 为 Go module `github.com/gospacex/webx`（go.mod + replace 指向本地 logx/configx，对齐 protocolx 惯例）
- [ ] 1.2 依赖锁定：`github.com/cloudwego/hertz`（版本对齐 protocolx/http 的 v0.10.x）、`go.opentelemetry.io/otel`、`google/uuid`；确认 `go build` 通过且依赖树无 `net/http` 服务端路径

## 2. envelope（统一响应 + 错误码）
- [ ] 2.1 `envelope/envelope.go`：`Response{Code, Message, Data, RequestID}` + `Render(c *app.RequestContext, data any, err error)`；`envelope.Error`（code+message，实现 error 与 Is）+ 分段错误码常量（1xxx/2xxx/4xxx/5xxx）+ `CodeToStatus` HTTP 映射表
- [ ] 2.2 单测：成功/失败渲染、`errors.Is` 判定、错误码表完整性（表驱动）

## 3. 中间件链
- [ ] 3.1 `middleware/recovery.go` + `middleware/timeout.go`：panic→5001/500、超时→5002/504，错误路径打 logx 日志
- [ ] 3.2 `middleware/ratelimit.go` + `middleware/cors.go`：令牌桶按 IP（429/4290）、预检放行/拒绝
- [ ] 3.3 `middleware/otel.go` + `middleware/logging.go`：root span（`<METHOD> <path>`）、trace-id 注入 ctx + `X-Request-Id` 响应头、access 日志（method/path/status/duration，caller 归因）
- [ ] 3.4 中间件默认链装配（recovery→otel→logx→ratelimit→cors→timeout→业务），`webx.WithMiddleware` 追加于链尾
- [ ] 3.5 单测：recovery/timeout/ratelimit/cors/otel/logx 各一功能用例 + 链顺序组合用例

## 4. webx bootstrap
- [ ] 4.1 `webx.go`：`App`（内嵌 `*hertz.App`）+ `New(opts...)` 选项（Config/WithLogger/WithTLS/WithMiddleware/WithReadyCheck/WithOTLP/WithMetrics）+ health 端点自动挂载（`/healthz/live`、`/healthz/ready`）
- [ ] 4.2 单例 `webx.App()`（首次默认配置 New 后缓存）+ `TestApp()`（独立实例、127.0.0.1:0 随机端口、`URL()` 取址）
- [ ] 4.3 优雅停机：`Shutdown(ctx)` 排空 in-flight；SIGINT/SIGTERM 装配示例（cmd 层）
- [ ] 4.4 单测：最小启动、缺省配置不 panic、单例同一指针、TestApp 随机端口、优雅停机排空

## 5. TLS / OTel
- [ ] 5.1 `WithTLS` 证书加载 + H2；加载失败 fail fast 返回含路径错误
- [ ] 5.2 OTel 装配：TracerProvider（OTLP gRPC）+ Meter（请求计数/耗时直方图/code 维度）；Shutdown flush
- [ ] 5.3 单测/集成：临时自签证书 HTTPS+h2 用例、span 导出断言、指标计数断言

## 6. examples 分层示例
- [ ] 6.1 目录骨架 + `internal/config`（configx 绑定业务配置）+ `internal/model`（entity + Repository 接口 + 内存实现）
- [ ] 6.2 `app/handler`（薄 handler）+ `app/routes.go`（注册源唯一）+ `internal/logic`（参数转换 + 调 service）
- [ ] 6.3 `internal/service`（业务核心 + 子 span）+ `internal/middleware`（鉴权业务中间件）+ `cmd/main.go`（一行启动）
- [ ] 6.4 各层单测（handler/logic/service/model，接口注入）
- [ ] 6.5 调用链开关：`internal/config` 增加 `Trace{Enabled, Exporter}`（configx 绑定 `demo.trace`，默认 false）；cmd 层开启时以 `OTel.Enabled=true` 构造 logx logger 并 `logx/zap.SetupTracerProvider` 装配 TracerProvider；`app/routes.go` 入口中间件开 root span（`<METHOD> <path>`），各层开子 span；开启/关闭两态各一测试用例（开启态断言 span 树 + 各层日志 trace_id 一致且挂 span event；关闭态断言无 span、行为不变）

## 7. 端到端与验收
- [ ] 7.1 端到端用例：调用链贯穿断言（请求 X-Request-Id == 响应 request_id == 三层日志 trace_id）
- [ ] 7.2 覆盖率：`go test -cover ./...` webx 核心三包 ≥ 80%；全量回归 `go test -race ./...` 绿
- [ ] 7.3 `openspec validate add-webx-hertz-skeleton --strict` 通过
