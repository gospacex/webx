# 实现计划：add-webx-hertz-skeleton

## 来源
- 提案：openspec/changes/add-webx-hertz-skeleton/proposal.md
- 设计：无（OpenSpec 判定无需，技术决策已并入 proposal「设计方向」）
- 规格：openspec/changes/add-webx-hertz-skeleton/specs/（webx-bootstrap / webx-middleware / webx-envelope / webx-observability / webx-examples）
- 任务：openspec/changes/add-webx-hertz-skeleton/tasks.md

## 实现步骤

> 按执行依赖排序：envelope 无外部依赖先行，中间件依赖 envelope，bootstrap 装配中间件，TLS/OTel 叠加在 bootstrap 之上，examples 依赖全部核心能力，端到端与验收殿后。

### Task 1: 模块脚手架与依赖
- [x] **任务完成**（与 superpowers plan `Task 1`、`tasks.md` 1.1/1.2 同步勾选）
- 目标：`hubx/webx` 初始化为 Go module `github.com/gospacex/webx`（含 git init），go.mod 锁定 hertz/logx/configx/otel/uuid 依赖（hertz 版本对齐 protocolx 的 v0.10.x），replace 指向本地 ../logx ../configx；全依赖树无 net/http 服务端路径
- 改动文件：`go.mod`、`go.sum`、git 初始化
- 验证方式：`go build ./...` 通过；`go mod graph | grep "net/http"` 无服务端栈

### Task 2: envelope 统一响应与错误码
- [x] **任务完成**（与 superpowers plan `Task 2`、`tasks.md` 2.1/2.2 同步勾选）
- 目标：`envelope.Response{Code,Message,Data,RequestID}` + `Render(c, data, err)`；`*Err`（code+msg，实现 error/Is）+ 分段错误码常量（1xxx/2xxx/4xxx/5xxx）+ `CodeToStatus` 映射；表驱动单测覆盖渲染、errors.Is、码表完整性
- 改动文件：`envelope/envelope.go`、`envelope/codes.go`、`envelope/envelope_test.go`
- 验证方式：`go test ./envelope/ -v` 全绿

### Task 3: 中间件链
- [x] **任务完成**（与 superpowers plan `Task 3`、`tasks.md` 3.1-3.5 同步勾选）
- 目标：recovery(panic→5001/500)、timeout(→5002/504)、ratelimit(令牌桶按 IP→429/4290)、cors(预检放行/拒绝)、otel(root span `<METHOD> <path>` + trace-id 注入 + X-Request-Id)、logx(access 日志 caller 归因)；默认链 recovery→otel→logx→ratelimit→cors→timeout→业务，WithMiddleware 追加链尾；每个中间件一个功能用例 + 链顺序组合用例
- 改动文件：`middleware/recovery.go`、`middleware/timeout.go`、`middleware/ratelimit.go`、`middleware/cors.go`、`middleware/otel.go`、`middleware/logging.go` + 对应 `_test.go`
- 验证方式：`go test ./middleware/ -v` 全绿

### Task 4: webx bootstrap 与 health
- [ ] **任务完成**（与 superpowers plan `Task 4`、`tasks.md` 4.1-4.4 同步勾选）
- 目标：`App`（内嵌 `*hertz` server）+ `New(opts...)` 选项（Config/WithLogger/WithTLS/WithMiddleware/WithReadyCheck/WithOTLP/WithMetrics）+ 自动挂载 `/healthz/live`、`/healthz/ready`；`webx.App()` 单例缓存 + `webx.TestApp()`（127.0.0.1:0 + `URL()`）；`Shutdown(ctx)` in-flight 排空 + SIGINT/SIGTERM 装配；单测覆盖最小启动/缺省配置/单例同指针/TestApp/优雅停机
- 改动文件：`webx.go`、`config.go`、`webx_test.go`
- 验证方式：`go test . -v` 全绿

### Task 5: TLS / OTel 集成
- [ ] **任务完成**（与 superpowers plan `Task 5`、`tasks.md` 5.1-5.3 同步勾选）
- 目标：`WithTLS` 证书加载 + H2，加载失败 fail fast 返回含路径错误；OTel TracerProvider（OTLP gRPC）+ Meter（请求计数/耗时直方图/code 维度），Shutdown flush；临时自签证书 HTTPS+h2 集成用例、span 导出断言、指标计数断言
- 改动文件：`webx.go`（TLS/OTel 装配）、`middleware/otel.go`（metrics）、`webx_tls_test.go`、`webx_otel_test.go`
- 验证方式：`go test . -run "TestTLS|TestOTel" -v` 全绿

### Task 6: examples 分层示例
- [ ] **任务完成**（与 superpowers plan `Task 6`、`tasks.md` 6.1-6.5 同步勾选）
- 目标：`examples/` 分层 demo —— `cmd/main.go`（configx+logx+webx 一行启动）、`app/handler`（薄 handler）+ `app/routes.go`（注册源唯一）、`internal/config`（业务配置含 `Trace{Enabled,Exporter}` 调用链开关）、`internal/model`（Order entity + Repository 接口 + 内存实现）、`internal/logic`（参数转换 + 调 service）、`internal/service`（业务核心 + 子 span + 2xxx 错误）、`internal/middleware`（鉴权）；**调用链开关**：开启时 cmd 以 `OTel.Enabled=true` 构造 logx 并经 `logx/zap.SetupTracerProvider` 装配 TracerProvider，`routes.go` 入口开 root span、各层开子 span 并经 logx context 方法入链（日志挂 span event + trace_id）；关闭态（默认）无 span、行为不变；各层单测（接口注入）+ 开关两态用例
- 改动文件：`examples/cmd/main.go`、`examples/app/**`、`examples/internal/**`、`examples/config.yaml`、`examples/internal/**/*_test.go`
- 验证方式：`go test ./examples/... -v` 全绿；`go run ./examples/cmd` 冒烟（开关开/关各一次）

### Task 7: 端到端与验收
- [ ] **任务完成**（与 superpowers plan `Task 7`、`tasks.md` 7.1-7.3 同步勾选）
- 目标：端到端"调用链贯穿"用例（请求 X-Request-Id == 响应 request_id == handler/service/model 三层日志 trace_id）；`go test -cover ./...` 核心三包 ≥ 80%、`go test -race ./...` 全量回归；`openspec validate add-webx-hertz-skeleton --strict` 通过
- 改动文件：`examples/e2e_test.go`、覆盖率报告
- 验证方式：覆盖率达标 + strict 校验输出 valid
