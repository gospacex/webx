# add-webx-hertz-skeleton

> **变更类型**: 新增独立 module
> **影响范围**: `hubx/webx`（新 module `github.com/gospacex/webx`），与 protocolx 解耦
> **日期**: 2026-09-16
> **驱动**: Hertz (`github.com/cloudwego/hertz`)

## 背景与目标

企业级 Go 服务端当前缺乏统一的 Web 初始化骨架：配置、日志、中间件、错误体系、可观测性、TLS、优雅停机分散在各业务工程，重复实现且行为不一致。

本次新增独立 module **`webx`**，基于 **Hertz**（不使用 `net/http`）提供企业级 Web 服务端初始化工具包，并附带分层示例工程，实现"一个请求全链路调用链可追踪、各层行为可测试"。

复用既有 gospacex 生态：**configx**（配置）、**logx**（日志 + OTel 调用链 + caller 归因）。

## 用户场景

1. 业务方一行 `cmd/main.go` 启动企业级 Web 服务（configx + logx + webx bootstrap）
2. 业务方通过 hz CLI 生成路由代码，业务逻辑落在 internal 分层，各层日志经 logx 调用链自动串联
3. 测试/多实例场景用 `webx.TestApp()` 起随机端口实例，端到端断言调用链贯通

## 设计方向

### 模块结构

```
webx/
├── webx.go                  # New(opts...) 显式构造 + App() 单例便捷入口 + TestApp() 逃生舱
├── middleware/              # 通用中间件链（recovery/timeout/cors/ratelimit/otel/logx）
├── envelope/                # 统一响应封装 + 分段错误码体系
└── examples/                # 企业级 web 分层 demo + 测试
    ├── cmd/main.go          # server 层：一行启动（configx+logx+webx）
    ├── app/                 # hz 生成区，不手工改
    │   ├── handler/         # 路由层：薄 handler，解析参数 → 调 logic
    │   └── routes.go        # 能生成则生成，否则移出，保证注册源唯一
    └── internal/
        ├── middleware/      # 业务中间件（鉴权等），复用 webx 中间件链
        ├── logic/           # 薄适配：参数转换 + 调 service
        ├── service/         # 业务核心（可选但推荐），开子 span
        ├── model/           # entity + Repository 接口 + demo 内存实现
        └── config/          # 业务侧配置结构（configx 绑定），与 webx: 段分离
```

### API 形态

- `webx.New(opts...)` 显式构造为主：`webx.Config(configx.Cfg)`、`webx.WithLogger(logx.Logger)`、`webx.WithTLS(cert, key)`、`webx.WithMiddleware(...)`；调用方拿到 App 后直接注册 Hertz 路由（与 hz 生成器协作）
- `webx.App()` 单例便捷入口：首次以默认配置 `New` 后缓存，对齐 protocolx 单例惯例
- `webx.TestApp()` 逃生舱：随机端口、跑完即释放，供多实例/测试场景

### 数据流 / 全链路调用约定

```
HTTP 请求
  → ① webx 中间件链: trace-id 注入 → otel 起 root span → 日志上下文
     （recovery / timeout / ratelimit / cors / auth(业务)）
  → ② handler(薄): 参数绑定 → 调 logic → envelope 渲染；禁止写业务，失败只透传
  → ③ logic(薄适配): 入参转换/校验 → 调 service → 出参裁剪
  → ④ service(业务核心): 领域规则、事务边界，开子 span
  → ⑤ model: 仓储接口在此定义，实现可换（内存 demo / GORM / 远程）

响应：统一 envelope {code, msg, data, request_id}
```

调用链串联：root span 在 otel 中间件开；logx `InfoContext` 等随 context 传播，
log 自动带 trace_id/span_id + 代码位置（zap caller skip 归因），
一条日志可追 handler→logic→service→model 整条链。

### 错误处理

原则：**错误在产生层定义，在中间件统一渲染，中间层只透传**。

| 层 | 职责 |
|---|---|
| model | 返回 `*envelope.Error`（code=1xxx 数据类），仓储错误即业务错误 |
| service | 领域规则错误（code=2xxx 业务类）；聚合/转换 model 错误；失败路径打 logx 日志（含 context） |
| logic/handler | 不定义错误，只透传；参数绑定失败 → `ErrBadRequest`（1001）；handler 末尾 `envelope.Render(c, err)` |
| 中间件 | recovery(panic→5001)、timeout(5002)、ratelimit(429/4290)、auth(401/4010) 统一翻成 envelope |

错误码分段（`envelope/codes.go`）：`1xxx` 通用/数据、`2xxx` 业务、`4xxx/5xxx` 网关语义；
`Code` 常量 + `Error()` 实现，支持 `errors.Is` 判定。不吞错：每条路径保证 request_id + error code 可追踪。

### 能力全集

- **Bootstrap**：configx 配置 + logx 日志 + 优雅启停（signal + drain，Shutdown 后 in-flight 排空）
- **中间件链**：recovery / timeout / cors / ratelimit / otel / logx / trace-id 注入 / auth 可插拔钩子
- **可观测性**：OpenTelemetry trace + metrics、health/readiness 端点
- **TLS/HTTPS**：证书加载、H2 支持
- **统一响应/错误**：envelope + 分段错误码

## 关键决策

1. `github.com/gospacex/webx` 独立 module，置于 `hubx/` 下，与 protocolx 解耦
2. 全复用 gospacex 生态：configx（配置）、logx（日志/调用链），webx 只做 Web 层
3. `New(opts...)` 显式构造为主，`App()` 单例降为薄便捷入口，`TestApp()` 保证可测试性（拒绝纯单例）
4. 路由走 hz CLI 生成器（B 方案）；业务中间件放 `internal/`（非 hz 生成区），避免生成器冲掉手写代码；`app/routes.go` 保证注册源唯一
5. 分层示例：handler(薄) → logic(薄适配) → service(业务核心) → model(entity+Repository 接口)，每层经 logx context 日志 + caller 归因串联调用链
6. 错误码分段体系 + 中间件统一渲染，中间层只透传
7. 测试目标：核心包覆盖率 ≥ 80%；端到端用例以"一条调用链贯穿"为核心断言（请求 trace-id 头 = 日志三层条目 = 响应 request_id）

## 范围边界

**包含：**
- webx 核心包（bootstrap / middleware / envelope）
- examples 分层示例工程（cmd/app/internal，含 hz 生成区约定）
- TLS、OTel、health、优雅停机
- 各层单测 + 端到端用例 + 中间件功能用例

**不包含（本次）：**
- gRPC / Kitex 协议：webx 功能矩阵仅覆盖 Hertz（HTTP）；gRPC 由 `protocolx/rpc`（Kitex 驱动）独立承接，与 webx 解耦，不在本变更范围
- 数据库/ORM 集成（model 只定义 Repository 接口 + 内存 demo 实现，GORM 留待后续）
- hz 生成器本身的开发（使用官方 hz CLI）
- 前端/网关/服务注册发现
- protocolx 驱动的改动（独立变更 protocolx-driver-update 覆盖）

## 验收标准

- [ ] `webx.New/...` 启动企业级 Hertz 服务，全链路不依赖 `net/http`
- [ ] examples 各层（handler/logic/service/model）单测通过，核心包覆盖率 ≥ 80%
- [ ] 端到端用例：请求 trace-id → 日志三层条目同 id → 响应 request_id 一致
- [ ] 中间件功能用例：recovery/timeout/ratelimit/auth/otel/logx 各一
- [ ] TLS/H2 用例通过；优雅停机 in-flight 排空用例通过
- [ ] 统一 envelope 渲染 + 错误码表完整，`errors.Is` 可判定
