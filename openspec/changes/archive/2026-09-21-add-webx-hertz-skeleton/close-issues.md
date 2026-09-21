# close-issues: add-webx-hertz-skeleton

close 阶段（2026-09-22）代码审查与规格一致性记录。

## 代码审查（close 阶段 2，code-reviewer 全量审查 d59babd..629ba10）

结论：0 CRITICAL / 1 HIGH / 6 MEDIUM / 7 LOW。

### H1（归档阻断，已修复）

**问题**：`middleware/otel.go` 未读取入站 `X-Request-Id` 请求头——规格
`specs/webx-middleware/spec.md`「Requirement: otel 与 trace-id 注入」Scenario
「trace-id 贯穿」明确 *WHEN 客户端带 `X-Request-Id` 请求头发起调用（或无头由
服务端生成）*，原实现无论有无入站头都自生成 trace-id，`examples/e2e_test.go`
对该场景属假绿。

**修复**（本 close 轮提交，close 阶段代码修复例外——阶段 2 审查发现的阻断项按
close.md「Critical/Important 终止 close → 修复后重新 close」处理）：
- `middleware/otel.go`：新增 `seedInboundRequestID`——入站 `X-Request-Id` 合法
  32-hex 时经 `trace.ContextWithRemoteSpanContext`（Remote=true, FlagsSampled）
  种子化，SDK 继承 trace-id、本地重生成 span-id；非法/缺失/全 0 → 保持原 ctx
  （SDK 生成），不静默吞
- `middleware/otel_test.go`：`TestOtelSeedsInboundRequestID`（合法入站贯穿 +
  非法入站服务端生成 + span trace-id 断言）
- `examples/e2e_test.go`：请求携带入站 `X-Request-Id`（固定 32-hex），断言链
  升级为「入站 X-Request-Id == 响应 X-Request-Id == body request_id == 三层日志
  trace_id」
- 计划文件 Task 7 注记与 Self-Review Open decision 行更新为已修复口径

**回归**：`go test ./...` 全绿；`go test -race ./...` 全绿；
`openspec validate add-webx-hertz-skeleton --strict` valid。

### MEDIUM / LOW（留档，不阻塞归档）

审查共 6 MEDIUM + 7 LOW，未逐条展开记录（reviewer 摘要输出），要点：
- demo 的 `Auth` 明文 token 校验属示例可接受，生产应接真实鉴权（LOW，示例标注）
- 后续可跟进：W3C `traceparent` 头传播（当前仅 32-hex `X-Request-Id`）；
  真 H2（`hertz-contrib/h2`）；OTLP gRPC 导出（离线约束，已列 open decision）

## 规格一致性（close 阶段 3 三维度对照）

| 维度 | 结果 |
|---|---|
| Completeness | tasks.md 24 项全 `[x]`；plan 文件 Step/Task complete/Self-Review 全勾；每 task 有实现+测试文件（1.3 盘点证据） |
| Correctness | H1 修复后 otel 中间件与 spec 场景「trace-id 贯穿」一致；e2e 断言闭环（入站头→响应头→body→三层日志） |
| Coherence | 无 design.md；代码遵循 proposal.md 分层（webx/middleware/envelope/examples）与项目既有 logx/configx 本地 replace 惯例；race 契约（WithPreRun 路由前置注册）为 hertz 约束落地，已记入计划追加实测块 |
