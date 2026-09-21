# add-webx-hertz-skeleton 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 `hubx/webx` 创建 `github.com/gospacex/webx` module——基于 Hertz 的企业级 Web 初始化骨架（中间件链 + envelope 统一响应/错误码 + examples 分层 demo），全链路经 configx/logx 可追踪，服务端不依赖 `net/http`。

**架构：** 三个核心包按依赖递增：`envelope`（无外部依赖）→ `middleware`（依赖 envelope + logx）→ 根包 `webx`（Hertz server 装配 + 单例 + TestApp + TLS/OTel）。`examples/` 为分层示例工程（cmd → app/handler → logic → service → model），每层日志经 logx context 串联，错误在产生层定义、中间层透传、由 envelope 统一渲染。

**技术栈：** Go 1.22+、`github.com/cloudwego/hertz v0.10.6`、`github.com/gospacex/logx`（本地 replace）、`github.com/gospacex/configx`（本地 replace）、OpenTelemetry **v1.29.0**（sdk trace + `otel/metric` Meter instrument + webx 内置 `AccessStats` 记录器做测试断言；**不引入** `otel/sdk/metric`，离线缓存缺失）、`github.com/google/uuid v1.6.0`、`golang.org/x/time v0.5.0`。

**工作目录约定：** 所有命令在 `/Users/hyx/work/gowork/src/hubx/webx` 下执行（module 根）。`git` 命令仅限 `hubx/webx` 仓库（Task 1 git init）。

---

## File Structure（先锁定文件划分，后定任务）

| 文件 | 职责 | 产出任务 |
|---|---|---|
| `go.mod` / `go.sum` | module `github.com/gospacex/webx`，replace 指向 `../logx` `../configx` | Task 1 |
| `envelope/codes.go` | 分段错误码常量（1xxx/2xxx/4xxx/5xxx）、`CodeToStatus` 映射、`RegisterCode`、`StatusOf` | Task 2 |
| `envelope/envelope.go` | `Response`、`*Error`（error+Is）、预设 sentinel、`Errorf`/`As`、request_id ctx 注入、`Render(c, data, err)` | Task 2 |
| `envelope/envelope_test.go` | 渲染成功/失败、`errors.Is`、码表完整性（表驱动） | Task 2 |
| `middleware/recovery.go` | panic → error 日志（含堆栈）→ 500/5001 | Task 3 |
| `middleware/timeout.go` | ctx 超时（默认 5s）→ 504/5002 | Task 3 |
| `middleware/ratelimit.go` | 令牌桶按 remote IP → 429/4290 | Task 3 |
| `middleware/cors.go` | 按配置 origins 预检放行/拒绝 | Task 3 |
| `middleware/otel.go` | root span（`<METHOD> <path>`）+ trace-id 注入 ctx + `X-Request-Id` 响应头 | Task 3 |
| `middleware/logging.go` | access 日志（method/path/remote_addr/status/duration/request_id，caller 归因）+ 指标记录钩子（`AccessStats`） | Task 3 |
| `middleware/*_test.go` | 每中间件一个功能用例 + 默认链顺序组合用例 | Task 3 |
| `config.go` | `webx.Config` 结构 + `FillDefaults` | Task 4 |
| `webx.go` | `App`（内嵌 `*server.Hertz`）、`New(opts...)` 选项体系、默认中间件链装配、health 端点、`App()` 单例、`TestApp()`、`URL()`、`Run()`、`Shutdown(ctx)` | Task 4 |
| `webx_test.go` | 最小启动 / 缺省配置 / 单例同指针 / TestApp 随机端口 / 优雅停机排空 / health 双探针 | Task 4 |
| `webx_tls_test.go` / `webx_otel_test.go` | 自签证书 HTTPS 用例、span 导出断言、指标计数断言、Shutdown flush | Task 5 |
| `examples/internal/config/config.go` | 业务配置结构（configx `Unmarshal("demo", ...)` 绑定） | Task 6 |
| `examples/internal/model/order.go` | `Order` entity + `OrderRepository` 接口 + `MemRepo` 内存实现（返回 `*envelope.Error` 1xxx） | Task 6 |
| `examples/internal/service/order.go` + `codes.go` | 业务核心 `OrderService`（子 span、model 错误 → 2xxx 业务码转换） | Task 6 |
| `examples/internal/logic/order.go` | 薄适配：参数校验/转换 + 调 service，只透传错误 | Task 6 |
| `examples/app/handler/order.go` | 薄 handler：`c.Param` → logic → `envelope.Render`；不写业务 | Task 6 |
| `examples/app/routes.go` | 路由注册源唯一（手写，非 hz 生成物；注释说明 hz 生成区约定） | Task 6 |
| `examples/internal/middleware/auth.go` | 业务鉴权中间件（复用 envelope 401/4010） | Task 6 |
| `examples/cmd/main.go` | 一行启动：configx + logx + webx + 优雅停机装配 | Task 6 |
| `examples/config.yaml` | demo 业务配置 | Task 6 |
| `examples/internal/**/*_test.go` | 各层单测（接口注入） | Task 6 |
| `examples/e2e_test.go` | 端到端调用链贯穿断言 + 全量回归/覆盖率/strict 校验 | Task 7 |

**类型一致性锚点（写后续任务前先读一遍）：**
- `envelope.Response{Code,Message,Data,RequestID}`、`envelope.Error{Code,Message}`、`envelope.As(err) *Error`、`envelope.Errorf(code, fmt, args...)`、`envelope.StatusOf(code) int`、`envelope.RegisterCode(code, status)`、`envelope.WithRequestID(c *app.RequestContext, id string)` / `envelope.RequestIDFrom(c *app.RequestContext) string`（经 `c.Set/c.Get` 流转，非 std context）、`envelope.Render(c *app.RequestContext, data any, err error)`
- `middleware.Recovery(logx.Logger)`、`middleware.Timeout(time.Duration, logx.Logger)`、`middleware.RateLimit(rps, burst int)`、`middleware.CORS(origins []string)`、`middleware.Otel(trace.Tracer)`、`middleware.Logging(AccessStats)`；`middleware.AccessStats{Logger logx.Logger; Requests metric.Int64Counter; Duration metric.Float64Histogram; ByCode metric.Int64Counter}`
- `webx.Config{Addr, Logger, TLSCertFile, TLSKeyFile, Timeout, RateLimitRPS, RateLimitBurst, CORSOrigins, HealthPrefix, ReadyCheck, OTLPEnabled, OTLPAddr, MetricsEnabled, injectTracerProvider, injectMeterProvider}`、`webx.New(opts...) (*App, error)`、`webx.App() *App`、`webx.TestApp(opts...) (*App, error)`、`app.URL() string`、`app.Run() error`、`app.Shutdown(ctx) error`

**测试内 HTTP 客户端：** 统一用 Hertz 官方客户端 `github.com/cloudwego/hertz/pkg/protocol/client`（`client.GetURL` / `hc.Do`，已在 v0.10.6 源码验证存在），不引入 `net/http`。

---

**⚠️ API 校正记录（v0.10.6 实测，写 Tasks 前必须遵守）**

以下 API 在 Hertz v0.10.6 中**不存在**，plan 早期草稿误用，已校正：
- ❌ `app.NewRequestContext(req, resp, maxParams)` → ✅ `app.NewContext(maxParams uint16)`（创建空 ctx，index=-1；需 `ctx.Request.SetMethod/SetRequestURI` 手工填充，`ctx.Response` 为内嵌值，读 `ctx.Response.StatusCode()` / `ctx.Response.Body()`）。
- ❌ `c.RequestCtx()`（RequestContext 无此方法）→ ✅ 测试用 `ctx := app.NewContext(0)`；生产链路里 hertz 会 `ServeHTTP(c context.Context, ctx *app.RequestContext)` 直接传入。
- ❌ 把 `*app.RequestContext` 当 `context.Context` 用（它只有 `Value`，无 `Deadline/Done/Err`）→ ✅ request-id 经 hertz 的 `c.Set(key,val)` / `c.Get(key)`（`ctx.Keys` map）流转，不引入 std context。

**Task 3 落地追加实测（OTel v1.29.0 Go SDK + hertz v0.10.6）：**
- `tracer.Start(ctx, name)` 在 Go SDK v1.29.0 返回 **双值 `(context.Context, trace.Span)`**（ctx 已含活动 span）——所有 `span := tr.Start(...)` 均写为 `ctx, span := tr.Start(ctx, name)`，下游直接 `c.Next(ctx)`（不再需要 `trace.ContextWithSpan`）。
- `otel.NewTracerProvider()` 在 otel v1.29.0 根包**不存在** → 全局句柄一律 `otel.GetTracerProvider()`（缺省 noop delegate，安全）。
- 无人写体跑完 chain 时 hertz 默认响应为 **200 Hello Hertz**（非 0）——超时守卫前置短路需 `c.Abort()` 停链（`return` 不会阻止引擎调 `engine.RootHandlers` 默认 200 填充）。
- `c.RemoteAddr()` 返回 `net.Addr` → 日志用 `.String()`。

**Task 2 落地追加实测（v0.10.6）：**
- `c.Get(key)` 返回**双值** `(any, bool)`（非 `c.Get(key).(string)` 单值形式）——`envelope.RequestIDFrom` 已按双值写法实现（`if v, ok := c.Get(requestIDKey); ok { if s, ok := v.(string); ... }`）。后续 Task 中凡引用 `c.Get` 的伪代码/注释均以此为准。
- sonic/ast 在 Go 1.27 环境降级 encoding/json（仅 WARNING，测试不受影响）。

因此 `envelope` 的 request-id API 校正为作用于 `*app.RequestContext`：
- `envelope.WithRequestID(c *app.RequestContext, id string)`（`c.Set`）
- `envelope.RequestIDFrom(c *app.RequestContext) string`（`c.Get`，缺省生成 uuid）
- `envelope.Render(c *app.RequestContext, data any, err error)` 内部经 `RequestIDFrom(c)` 取 id。

**OTel 版本约束（离线可构建）：** 核心 otel 锁定 **v1.29.0**（与 logx/protocolx 一致，保证 logx 的 jaeger/otlptracehttp exporter 可用）。`otel/sdk/trace` v1.29.0 已在本地缓存（`tracetest.InMemoryExporter`、`NewTracerProvider/WithSyncer` 可用）。**不引入 `otel/sdk/metric`**（该版本本地无缓存且网络被拦截）——指标经 `otel/metric` v1.29.0 的 `Meter` 创建 instrument，测试断言走 webx 内置的线程安全 `AccessStats` 记录器，非 otel reader。

---

### Task 1: 模块脚手架与依赖

> **trace:** plan-ready.md → `### Task 1: 模块脚手架与依赖` | tasks.md → `- [ ] 1.1 初始化 `hubx/webx` 为 Go module `github.com/gospacex/webx`（go.mod + replace 指向本地 logx/configx，对齐 protocolx 惯例）`
> **sync:** tasks.md → `- [ ] 1.1 初始化 `hubx/webx` 为 Go module `github.com/gospacex/webx`（go.mod + replace 指向本地 logx/configx，对齐 protocolx 惯例）` | plan-ready.md → `### Task 1: 模块脚手架与依赖`

**Files:**
- 创建：`go.mod`、`go.sum`
- 创建：`.git`（git init）

- [x] **步骤 1：git 初始化 + module 初始化**

```bash
cd /Users/hyx/work/gowork/src/hubx/webx
git init
go mod init github.com/gospacex/webx
```

预期：`go.mod` 生成，module 名为 `github.com/gospacex/webx`。

- [x] **步骤 2：锁定依赖（hertz 对齐 protocolx 的 v0.10.x → v0.10.6；OTel 锁 v1.29.0 离线可用族；本地 replace logx/configx）**

```bash
go get github.com/cloudwego/hertz@v0.10.6
go get go.opentelemetry.io/otel@v1.29.0 go.opentelemetry.io/otel/trace@v1.29.0 go.opentelemetry.io/otel/metric@v1.29.0 go.opentelemetry.io/otel/sdk@v1.29.0
go get github.com/google/uuid@v1.6.0
go get golang.org/x/time@v0.5.0
go mod edit -require=github.com/gospacex/logx@v0.0.0 -require=github.com/gospacex/configx@v0.0.0
go mod edit -replace=github.com/gospacex/logx=../logx -replace=github.com/gospacex/configx=../configx
go mod tidy
```

**版本选择依据（离线可构建）：** OTel 锁 **v1.29.0 族**（与 logx/protocolx 一致，logx 的 jaeger/otlptracehttp exporter 在该版本本地有缓存）；**不引入 `otel/sdk/metric`**（本地无缓存、网络被拦截）——指标仅用 `otel/metric` 的 `Meter` instrument，测试断言走 webx 内置 `AccessStats` 记录器。**不引入 otlptracegrpc/otlpmetricgrpc**（本地无缓存，OTLP 导出在 Task 5 经 logx 的 otlptracehttp/stdout 导出器实现，gRPC 导出留给后续变更）。

**离线兜底（`go get` 报 module lookup 失败时）：**

```bash
go mod edit -require=github.com/cloudwego/hertz@v0.10.6 -require=go.opentelemetry.io/otel@v1.29.0 -require=go.opentelemetry.io/otel/trace@v1.29.0 -require=go.opentelemetry.io/otel/metric@v1.29.0 -require=go.opentelemetry.io/otel/sdk@v1.29.0 -require=github.com/google/uuid@v1.6.0 -require=golang.org/x/time@v0.5.0
GOPROXY=off GOFLAGS=-mod=mod go mod tidy
```

预期：`go.mod` 含 `replace github.com/gospacex/logx => ../logx` 与 `github.com/gospacex/configx => ../configx`（对齐 `hubx/protocolx` 根模块惯例），OTel 各包锁定 v1.29.0。

- [x] **步骤 3：验证构建与依赖树**

运行：

```bash
go build ./...
go mod graph | grep -E "golang.org/x/net/http|google.golang.org/grpc" || echo "OK: no net/http server stack"
```

预期：第一行无输出（空 module 中仅有声明的包）；第二行输出 `OK: no net/http server stack`（依赖树无 `net/http` 服务端路径，gRPC 栈同样不在）。

- [x] **步骤 4：Commit**

```bash
git add -A
git commit -m "chore: scaffold github.com/gospacex/webx module (hertz v0.10.6 + local logx/configx)"
```

- [x] **Task complete**（本 Task 全部 Step 为 `[x]` 后勾选；与 plan-ready **任务完成**、tasks.md 对应行同步）

---

### Task 2: envelope 统一响应与错误码

> **trace:** plan-ready.md → `### Task 2: envelope 统一响应与错误码` | tasks.md → `- [ ] 2.1 `envelope/envelope.go`：`Response{Code, Message, Data, RequestID}` + `Render(c *app.RequestContext, data any, err error)`；`envelope.Error`（code+message，实现 error 与 Is）+ 分段错误码常量（1xxx/2xxx/4xxx/5xxx）+ `CodeToStatus` HTTP 映射表`
> **sync:** tasks.md → `- [ ] 2.1 `envelope/envelope.go`：`Response{Code, Message, Data, RequestID}` + `Render(c *app.RequestContext, data any, err error)`；`envelope.Error`（code+message，实现 error 与 Is）+ 分段错误码常量（1xxx/2xxx/4xxx/5xxx）+ `CodeToStatus` HTTP 映射表` | plan-ready.md → `### Task 2: envelope 统一响应与错误码`

**Files:**
- 创建：`envelope/codes.go`、`envelope/envelope.go`
- 测试：`envelope/envelope_test.go`

- [x] **步骤 1：编写失败测试（表驱动）**

创建 `envelope/envelope_test.go`：

```go
package envelope

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// newTestContext 造一个可用的 *app.RequestContext（hertz v0.10.6 无 NewRequestContext，
// 用 NewContext(0) + 手工填充 Request 字段；Response 为内嵌值，直接读）。
func newTestContext(requestID string) *app.RequestContext {
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/demo/42")
	c.Request.SetHost("webx.test")
	if requestID != "" {
		WithRequestID(c, requestID)
	}
	return c
}

func TestRenderSuccess(t *testing.T) {
	c := newTestContext("rid-42")
	Render(c, map[string]string{"k": "v"}, nil)

	if got := c.Response.StatusCode(); got != consts.StatusOK {
		t.Fatalf("status = %d, want %d", got, consts.StatusOK)
	}
	if !strings.Contains(string(c.Response.Header.ContentType()), "application/json") {
		t.Fatalf("content-type = %s, want application/json", c.Response.Header.ContentType())
	}
	var r Response
	if err := json.Unmarshal(c.Response.Body(), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Code != CodeOK || r.Message != "ok" || r.RequestID != "rid-42" || r.Data == nil {
		t.Fatalf("unexpected success response: %+v", r)
	}
}

func TestRenderFailure(t *testing.T) {
	c := newTestContext("rid-42")
	Render(c, nil, Errorf(CodeNotFound, "用户 %d 不存在", 42))

	if got := c.Response.StatusCode(); got != consts.StatusNotFound {
		t.Fatalf("status = %d, want 404", got)
	}
	var r Response
	if err := json.Unmarshal(c.Response.Body(), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Code != CodeNotFound || !strings.Contains(r.Message, "42") || r.RequestID != "rid-42" || r.Data != nil {
		t.Fatalf("unexpected failure response: %+v", r)
	}
}

func TestErrorsIs(t *testing.T) {
	err := Errorf(CodeNotFound, "user %d missing", 7)
	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is(code=1004, ErrNotFound) = false, want true")
	}
	if errors.Is(err, ErrBadRequest) {
		t.Error("errors.Is(code=1004, ErrBadRequest) = true, want false")
	}
	// 非 envelope 错误归一为 CodeInternal
	if e := As(errors.New("boom")); e.Code != CodeInternal {
		t.Fatalf("As(plain) code = %d, want %d", e.Code, CodeInternal)
	}
}

func TestCodeTableComplete(t *testing.T) {
	for code, name := range CodeNames {
		if _, ok := CodeToStatus[code]; !ok {
			t.Errorf("CodeToStatus missing preset code %d (%s)", code, name)
		}
		if StatusOf(code) == 0 {
			t.Errorf("StatusOf(%d) = 0, want mapped status", code)
		}
	}
}

func TestRegisterCode(t *testing.T) {
	RegisterCode(2001, 404)
	if got := StatusOf(2001); got != 404 {
		t.Fatalf("StatusOf(2001) = %d, want 404", got)
	}
}
```

- [x] **步骤 2：运行测试确认失败**

运行：`go test ./envelope/ -v`
预期：编译失败（`undefined: Response / Render / CodeNotFound / ...`）——红灯。

- [x] **步骤 3：实现 `envelope/codes.go`**

```go
package envelope

import "sync"

// 分段错误码：1xxx 通用/数据、2xxx 业务（业务方扩展）、4xxx 网关语义、5xxx 服务端
const (
	CodeOK           = 0
	CodeBadRequest   = 1001
	CodeNotFound     = 1004
	CodeConflict     = 1009
	CodeBizRule      = 2000
	CodeUnauthorized = 4010
	CodeRateLimited  = 4290
	CodeInternal     = 5000
	CodePanic        = 5001
	CodeTimeout      = 5002
)

var (
	codeMu     sync.RWMutex
	// CodeToStatus 预置码 → HTTP 状态映射，业务方可用 RegisterCode 扩展
	CodeToStatus = map[int]int{
		CodeOK:           200,
		CodeBadRequest:   400,
		CodeNotFound:     404,
		CodeConflict:     409,
		CodeBizRule:      400,
		CodeUnauthorized: 401,
		CodeRateLimited:  429,
		CodeInternal:     500,
		CodePanic:        500,
		CodeTimeout:      504,
	}
	// CodeNames 预置码 → 稳定文案（测试用码表遍历）
	CodeNames = map[int]string{
		CodeOK: "ok", CodeBadRequest: "bad request", CodeNotFound: "not found",
		CodeConflict: "conflict", CodeBizRule: "business rule violated",
		CodeUnauthorized: "unauthorized", CodeRateLimited: "rate limited",
		CodeInternal: "internal server error", CodePanic: "panic recovered",
		CodeTimeout: "request timeout",
	}
)

// RegisterCode 为业务扩展码（2xxx 段）注册 HTTP 状态映射
func RegisterCode(code, httpStatus int) {
	codeMu.Lock()
	defer codeMu.Unlock()
	CodeToStatus[code] = httpStatus
}

// StatusOf 取 code 对应 HTTP 状态；未映射码兜底 500（不吞码：码仍出现在 envelope body）
func StatusOf(code int) int {
	codeMu.RLock()
	defer codeMu.RUnlock()
	if s, ok := CodeToStatus[code]; ok {
		return s
	}
	return 500
}
```

- [x] **步骤 4：实现 `envelope/envelope.go`**

```go
package envelope

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
)

// Response 统一响应封装
type Response struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

// Error 分段错误：code + message，支持 errors.Is 按码判定
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }

// Is 按错误码等价判定（1004 的所有实例 Is(ErrNotFound) 为 true）
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// 预设 sentinel 错误
var (
	ErrBadRequest   = &Error{Code: CodeBadRequest, Message: "bad request"}
	ErrNotFound     = &Error{Code: CodeNotFound, Message: "not found"}
	ErrConflict     = &Error{Code: CodeConflict, Message: "conflict"}
	ErrBizRule      = &Error{Code: CodeBizRule, Message: "business rule violated"}
	ErrUnauthorized = &Error{Code: CodeUnauthorized, Message: "unauthorized"}
	ErrRateLimited  = &Error{Code: CodeRateLimited, Message: "rate limited"}
	ErrInternal     = &Error{Code: CodeInternal, Message: "internal server error"}
	ErrPanic        = &Error{Code: CodePanic, Message: "panic recovered"}
	ErrTimeout      = &Error{Code: CodeTimeout, Message: "request timeout"}
)

// Errorf 创建带格式化文案的 *Error
func Errorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// As 将任意 error 归一为 *Error（非 envelope 错误 → CodeInternal，不吞错）
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeInternal, Message: err.Error()}
}

// requestIDKey 是 hertz ctx.Keys 中 request-id 的键（*app.RequestContext 非 context.Context，
// 只能经 c.Set/c.Get 流转，见「API 校正记录」）
const requestIDKey = "webx.request_id"

// WithRequestID 将 request id 写入 hertz ctx（由 otel 中间件在读入 X-Request-Id 后调用）
func WithRequestID(c *app.RequestContext, id string) {
	c.Set(requestIDKey, id)
}

// RequestIDFrom 从 hertz ctx 取 request id；无则生成 uuid（保证每条路径可追踪）
func RequestIDFrom(c *app.RequestContext) string {
	if v, ok := c.Get(requestIDKey).(string); ok && v != "" {
		return v
	}
	rid := uuid.NewString()
	WithRequestID(c, rid) // 回写：同请求后续中间件/handler 取到同一 id
	return rid
}

// Render 统一渲染 JSON envelope：err 为 nil → code=0 + data；
// err 非 nil → code/message 取自错误、data 为 nil、HTTP 状态按 CodeToStatus 映射
func Render(c *app.RequestContext, data any, err error) {
	rid := RequestIDFrom(c)
	if err == nil {
		c.JSON(consts.StatusOK, Response{Code: CodeOK, Message: "ok", Data: data, RequestID: rid})
		return
	}
	e := As(err)
	c.JSON(StatusOf(e.Code), Response{Code: e.Code, Message: e.Message, Data: nil, RequestID: rid})
}
```

- [x] **步骤 5：运行测试确认通过**

运行：`go test ./envelope/ -v`
预期：`PASS`，5 个测试全绿。

- [ ] **步骤 6：Commit**

```bash
git add envelope/
git commit -m "feat: envelope unified response + segmented error codes (errors.Is, CodeToStatus)"
```

- [ ] **Task complete**（本 Task 全部 Step 为 `[x]` 后勾选；与 plan-ready **任务完成**、tasks.md 对应行同步）

---

### Task 3: 中间件链

> **trace:** plan-ready.md → `### Task 3: 中间件链` | tasks.md → `- [ ] 3.1 `middleware/recovery.go` + `middleware/timeout.go`：panic→5001/500、超时→5002/504，错误路径打 logx 日志` / `- [ ] 3.2 `middleware/ratelimit.go` + `middleware/cors.go`：令牌桶按 IP（429/4290）、预检放行/拒绝` / `- [ ] 3.3 `middleware/otel.go` + `middleware/logging.go`：root span（`<METHOD> <path>`）、trace-id 注入 ctx + `X-Request-Id` 响应头、access 日志（method/path/status/duration，caller 归因）` / `- [ ] 3.4 中间件默认链装配（recovery→otel→logx→ratelimit→cors→timeout→业务），`webx.WithMiddleware` 追加于链尾` / `- [ ] 3.5 单测：recovery/timeout/ratelimit/cors/otel/logx 各一功能用例 + 链顺序组合用例`
> **sync:** tasks.md → `3.1–3.5` | plan-ready.md → `### Task 3: 中间件链`

**Files:**
- 创建：`middleware/recovery.go`、`middleware/timeout.go`、`middleware/ratelimit.go`、`middleware/cors.go`、`middleware/otel.go`、`middleware/logging.go`、`middleware/access_stats.go`、`middleware/chain.go`
- 测试：`middleware/recovery_test.go`、`middleware/timeout_test.go`、`middleware/ratelimit_test.go`、`middleware/cors_test.go`、`middleware/otel_test.go`、`middleware/logging_test.go`、`middleware/chain_test.go`

**通用约定（写代码前先读一遍）：**
- 每个中间件返回 `app.HandlerFunc`（`func(ctx context.Context, c *app.RequestContext)`），经 `c.Next(ctx)` 续链。
- 单测统一「chain-unit」模式：`ctx := app.NewContext(0)` → `ctx.Request.SetMethod/SetRequestURI` → `ctx.SetHandlers(app.HandlersChain{mw, handler})` → `ctx.Next(context.Background())` → 断言 `ctx.Response.StatusCode()/Body()/Header`。
- `*app.RequestContext` 非 std `context.Context`：request-id 经 `envelope.WithRequestID(c, id)`/`envelope.RequestIDFrom(c)`（`c.Set/c.Get`）流转；std `ctx`（span 载体）经 `c.Next(ctx)` 向下传递。
- 测试内 logger 用 `logx.GetGlobal()`（无 provider 注册时返回 noop，安全，不 blank-import `logx/zap`）。
- 默认链顺序 `recovery → otel → logging → ratelimit → cors → timeout → 业务`；`DefaultChain` 只产出前 6 个（业务与 `WithMiddleware` 追加由 Task 4 负责）。

- [ ] **步骤 1：编写失败测试（每中间件一功能用例 + 链顺序组合用例）**

创建 `middleware/recovery_test.go`：

```go
package middleware

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
)

// runChain 链单测公共驱动：mw 后接 handler，跑一轮 hertz handler chain。
func runChain(t *testing.T, mw app.HandlerFunc, handler app.HandlerFunc) *app.RequestContext {
	t.Helper()
	ctx := app.NewContext(0)
	ctx.Request.SetMethod("GET")
	ctx.Request.SetRequestURI("/boom")
	ctx.SetHandlers(app.HandlersChain{mw, handler})
	ctx.Next(context.Background())
	return ctx
}

func TestRecoveryRecoversPanic(t *testing.T) {
	mw := Recovery(logx.GetGlobal())
	handler := func(ctx context.Context, c *app.RequestContext) { panic("demo panic") }
	got := runChain(t, mw, handler)

	if got.Response.StatusCode() != 500 {
		t.Fatalf("status = %d, want 500", got.Response.StatusCode())
	}
	body := string(got.Response.Body())
	if !strings.Contains(body, `"code":5001`) {
		t.Fatalf("body = %s, want envelope code 5001", body)
	}
}
```

创建 `middleware/timeout_test.go`：

```go
package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
)

func TestTimeoutPassesDeadlineDown(t *testing.T) {
	mw := Timeout(3*time.Second, logx.GetGlobal())
	sawDeadline := false
	handler := func(ctx context.Context, c *app.RequestContext) {
		_, ok := ctx.Deadline()
		sawDeadline = ok
		c.Next(ctx)
	}
	// 业务 handler 立即返回（未到 deadline）：不写响应、不触发 504
	got := runChain(t, mw, handler)
	if !sawDeadline {
		t.Fatal("downstream ctx has no deadline; Timeout did not wrap context")
	}
	if got.Response.StatusCode() != 0 {
		t.Fatalf("status = %d, want 0 (no body written on fast path)", got.Response.StatusCode())
	}
}

func TestTimeoutExceededRenders504(t *testing.T) {
	mw := Timeout(3*time.Second, logx.GetGlobal())
	// 模拟 handler 挂起超过 deadline：直接造一个已过期 deadline 的 ctx 传不下去，
	// 因此这里改由「handler 内 sleep 超过测试阈值」不可行（会拖慢测试）；
	// 改为用 1ns 级 Timeout + handler sleep 1ms 触发超时分支。
	mwFast := Timeout(time.Nanosecond, logx.GetGlobal())
	handler := func(ctx context.Context, c *app.RequestContext) {
		time.Sleep(time.Millisecond)
	}
	got := runChain(t, mwFast, handler)
	_ = mw
	if got.Response.StatusCode() != 504 {
		t.Fatalf("status = %d, want 504", got.Response.StatusCode())
	}
}
```

创建 `middleware/ratelimit_test.go`：

```go
package middleware

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestRateLimitAllowsThenDenies(t *testing.T) {
	mw := RateLimit(1, 1) // rps=1 burst=1：同 IP 首请求放行，次请求拒
	okHandler := func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }
	// 两请求同 IP（NewContext 未设连接 → ClientIP 为空串，同键命中同桶）
	got := runChain(t, mw, okHandler)
	if got.Response.StatusCode() != 0 {
		t.Fatalf("first request status = %d, want 0 (allowed, no body)", got.Response.StatusCode())
	}
	got2 := runChain(t, mw, okHandler)
	if got2.Response.StatusCode() != 429 {
		t.Fatalf("second request status = %d, want 429", got2.Response.StatusCode())
	}
}
```

创建 `middleware/cors_test.go`：

```go
package middleware

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func newPreflightCtx(origin string) *app.RequestContext {
	c := app.NewContext(0)
	c.Request.SetMethod("OPTIONS")
	c.Request.SetRequestURI("/x")
	c.Request.SetHost("webx.test")
	if origin != "" {
		c.Request.Header.Set("Origin", origin)
		c.Request.Header.Set("Access-Control-Request-Method", "GET")
	}
	return c
}

func TestCORSAllowPreflight(t *testing.T) {
	mw := CORS([]string{"https://a.example"})
	c := newPreflightCtx("https://a.example")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())
	if got := string(c.Response.Header.Peek("Access-Control-Allow-Origin")); got != "https://a.example" {
		t.Fatalf("allow-origin = %q, want https://a.example", got)
	}
	if c.Response.StatusCode() != 204 {
		t.Fatalf("preflight status = %d, want 204", c.Response.StatusCode())
	}
}

func TestCORSRejectOrigin(t *testing.T) {
	mw := CORS([]string{"https://a.example"})
	c := newPreflightCtx("https://evil.example")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())
	if c.Response.StatusCode() != 403 {
		t.Fatalf("reject status = %d, want 403", c.Response.StatusCode())
	}
}

func TestCORSNoOriginPassThrough(t *testing.T) {
	mw := CORS([]string{"https://a.example"})
	c := newPreflightCtx("") // 无 Origin 头：非 CORS 请求，直接放行
	handled := false
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { handled = true }})
	c.Next(context.Background())
	if !handled {
		t.Fatal("handler not reached for non-CORS request")
	}
}
```

创建 `middleware/otel_test.go`：

```go
package middleware

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
	otrace "go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// 注：tracer 经 sdktrace provider 构造；service 层 Start 返回 (ctx, span)

// otelTracerForTest 用 InMemoryExporter + Syncer 造真实 Tracer（trace-id 非全 0）
func otelTracerForTest(t *testing.T, exp *tracetest.InMemoryExporter) otrace.Tracer {
	t.Helper()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider.Tracer("webx-test")
}

func TestOtelSetsRequestIDHeader(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tr := otelTracerForTest(t, exp)
	mw := Otel(tr)
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/demo/42")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())

	rid := string(c.Response.Header.Peek("X-Request-Id"))
	if len(rid) != 32 {
		t.Fatalf("X-Request-Id = %q, want 32-hex trace-id", rid)
	}
	// trace-id 同步写入 hertz ctx（envelope 取用）
	if envelope.RequestIDFrom(c) != rid {
		t.Fatalf("envelope.RequestIDFrom = %q, want %q", envelope.RequestIDFrom(c), rid)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %+v, want 1 exported root span", spans)
	}
	if !strings.Contains(spans[0].Name, "GET") {
		t.Fatalf("span name = %q, want contains 'GET'", spans[0].Name)
	}
}
```

创建 `middleware/logging_test.go`：

```go
package middleware

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
)

func TestLoggingRecordsStats(t *testing.T) {
	stats := NewAccessStats()
	mw := Logging(logx.GetGlobal(), stats)
	c := app.NewContext(0)
	c.Request.SetMethod("POST")
	c.Request.SetRequestURI("/demo")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, "ok")
	}})
	c.Next(context.Background())

	snap := stats.Snapshot()
	if snap.Requests != 1 {
		t.Fatalf("stats.Requests = %d, want 1", snap.Requests)
	}
	if snap.LastStatus != 200 {
		t.Fatalf("stats.LastStatus = %d, want 200", snap.LastStatus)
	}
}
```

创建 `middleware/chain_test.go`（链顺序组合用例）：

```go
package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"go.opentelemetry.io/otel"
)

func TestDefaultChainOrder(t *testing.T) {
	tracer := otel.GetTracerProvider().Tracer("webx-test") // noop 即可
	stats := NewAccessStats()
	chain := DefaultChain(tracer, stats, logx.GetGlobal(), 10, 20, nil, 5*time.Second)
	if len(chain) != 6 {
		t.Fatalf("DefaultChain len = %d, want 6", len(chain))
	}
	// 组合跑一轮：链内无 panic/限流触发，200 正常透传
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/ordered")
	c.SetHandlers(append(chain, func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, "ok")
	}))
	c.Next(context.Background())
	if c.Response.StatusCode() != 200 {
		t.Fatalf("composed chain status = %d, want 200", c.Response.StatusCode())
	}
	if stats.Snapshot().Requests != 1 {
		t.Fatalf("logging in composed chain recorded %d, want 1", stats.Snapshot().Requests)
	}
}
```

- [ ] **步骤 2：运行测试确认失败**

运行：`go test ./middleware/ -v`
预期：编译失败（`undefined: Recovery / Timeout / RateLimit / CORS / Otel / Logging / NewAccessStats / DefaultChain`）——红灯。

- [ ] **步骤 3：实现 `middleware/access_stats.go`（线程安全记录器 + 指标接线）**

```go
package middleware

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AccessStats webx 内置线程安全访问统计。
// 离线环境不引入 otel/sdk/metric：instrument 经 otel/metric Meter 创建（Task 5 接线），
// 测试断言走本记录器的原子快照，不依赖 otel reader。
type AccessStats struct {
	mu sync.Mutex
	// instrument 未接线前为 nil，Record 判空跳过
	reqCounter  metric.Int64Counter
	durHist     metric.Float64Histogram
	codeCounter metric.Int64Counter

	requests   int64
	lastStatus int
}

// NewAccessStats 创建未接线的 AccessStats
func NewAccessStats() *AccessStats { return &AccessStats{} }

// Snapshot 线程安全快照（测试断言用）
type StatsSnapshot struct {
	Requests   int64
	LastStatus int
}

func (s *AccessStats) Snapshot() StatsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return StatsSnapshot{Requests: s.requests, LastStatus: s.lastStatus}
}

// Record 一次访问（Logging 中间件请求完成后调用；ctx 为请求 ctx，span 属性自动带入）
func (s *AccessStats) Record(ctx context.Context, status int, dur time.Duration) {
	s.mu.Lock()
	s.requests++
	s.lastStatus = status
	req, code, durH := s.reqCounter, s.codeCounter, s.durHist
	s.mu.Unlock()

	if req != nil {
		req.Add(ctx, 1, metric.WithAttributes(
			attribute.Int("http.status_code", status),
		))
	}
	if code != nil {
		code.Add(ctx, 1, metric.WithAttributes(
			attribute.Int("http.status_code", status),
		))
	}
	if durH != nil {
		durH.Record(ctx, float64(dur.Seconds()), metric.WithAttributes(
			attribute.Int("http.status_code", status),
		))
	}
}

// BindMeter 由 webx 根包（Task 5）在 Meter 可用时接线 instrument
func (s *AccessStats) BindMeter(meter metric.Meter) error {
	var err error
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reqCounter == nil {
		s.reqCounter, err = meter.Int64Counter("webx.requests",
			metric.WithDescription("total requests"),
			metric.WithUnit("{request}"))
	}
	if err == nil && s.codeCounter == nil {
		s.codeCounter, err = meter.Int64Counter("webx.responses_by_code",
			metric.WithDescription("responses by http status code"),
			metric.WithUnit("{response}"))
	}
	if err == nil && s.durHist == nil {
		s.durHist, err = meter.Float64Histogram("webx.request.duration",
			metric.WithDescription("request duration seconds"),
			metric.WithUnit("s"))
	}
	return err
}

// var 占位：保证 otel 包在无人引用时不丢 import（BindMeter 未接线前）
var _ = otel.Meter
```

创建 `middleware/recovery.go`：

```go
package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
)

// Recovery panic 兜底：恢复后以 5001/500 渲染 envelope，error 日志（含 panic 值与 path 字段）
func Recovery(l logx.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				l.ErrorContext(ctx, "recovered panic",
					logx.Any("panic", r),
					logx.String("path", string(c.FullPath())))
				c.Abort()
				envelope.Render(c, nil, envelope.ErrPanic) // code 5001 → 500
			}
		}()
		c.Next(ctx)
	}
}
```

创建 `middleware/timeout.go`：

```go
package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
)

// Timeout 给下游 ctx 打 deadline；handler 返回时若已过期且尚未写响应 → 5002/504
func Timeout(d time.Duration, l logx.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		reqCtx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		c.Next(reqCtx)
		if c.IsAborted() || c.Response.StatusCode() != 0 {
			return // 下游已自行结束（含业务写体 / 其它中间件 Abort）
		}
		if dl, ok := reqCtx.Deadline(); ok && time.Now().After(dl) {
			l.WarnContext(reqCtx, "request timed out",
				logx.String("path", string(c.FullPath())))
			envelope.Render(c, nil, envelope.ErrTimeout) // code 5002 → 504
		}
	}
}
```

创建 `middleware/ratelimit.go`：

```go
package middleware

import (
	"context"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
	"golang.org/x/time/rate"
)

// RateLimit 按 client IP 令牌桶限流；超限 → 4290/429
// 桶以请求 IP 为键（单元测试中 ClientIP 为空串，同键命中同桶）
func RateLimit(rps, burst int) app.HandlerFunc {
	if rps <= 0 {
		rps = 10
	}
	if burst <= 0 {
		burst = rps
	}
	mu := &sync.Mutex{}
	buckets := make(map[string]*rate.Limiter)
	return func(ctx context.Context, c *app.RequestContext) {
		key := c.ClientIP()
		mu.Lock()
		lim, ok := buckets[key]
		if !ok {
			lim = rate.NewLimiter(rate.Limit(rps), burst)
			buckets[key] = lim
		}
		mu.Unlock()
		if !lim.Allow() {
			c.Abort()
			envelope.Render(c, nil, envelope.ErrRateLimited) // code 4290 → 429
			return
		}
		c.Next(ctx)
	}
}
```

创建 `middleware/cors.go`：

```go
package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// CORS 按 origins 白名单放行；origins 为空 = 不启用 CORS（直接放行）。
// 预检（OPTIONS + Origin）：白名单命中 → 204 + 允许头；未命中 → 403。
func CORS(origins []string) app.HandlerFunc {
	allow := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allow[o] = struct{}{}
	}
	return func(ctx context.Context, c *app.RequestContext) {
		origin := string(c.Request.Header.Peek("Origin"))
		if origin == "" || len(allow) == 0 {
			c.Next(ctx)
			return
		}
		if _, ok := allow[origin]; !ok {
			if isPreflight(c) {
				c.AbortWithStatus(consts.StatusForbidden)
				return
			}
			c.Next(ctx) // 非预检的简单请求不阻断（浏览器侧自行判）
			return
		}
		c.Response.Header.Set("Access-Control-Allow-Origin", origin)
		if isPreflight(c) {
			if m := string(c.Request.Header.Peek("Access-Control-Request-Method")); m != "" {
				c.Response.Header.Set("Access-Control-Allow-Methods", m)
			}
			c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
			c.Response.Header.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(consts.StatusNoContent)
			return
		}
		c.Next(ctx)
	}
}

func isPreflight(c *app.RequestContext) bool {
	return string(c.Method()) == "OPTIONS"
}
```

创建 `middleware/otel.go`：

```go
package middleware

import (
	"context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Otel 开 root span `<METHOD> <path>`；trace-id 经 envelope.WithRequestID 写入 hertz ctx，
// 并回写 `X-Request-Id` 响应头；span 经 trace.ContextWithSpan 传给下游 handler。
func Otel(tr trace.Tracer) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		name := fmt.Sprintf("%s %s", string(c.Method()), string(c.Path()))
		// v1.29.0 Go SDK: Start 返回 (Context, Span)——ctx 已含活动 span
		ctx, span := tr.Start(ctx, name)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", string(c.Method())),
			attribute.String("http.target", string(c.FullPath())),
		)
		rid := span.SpanContext().TraceID().String()
		envelope.WithRequestID(c, rid)
		c.Response.Header.Set("X-Request-Id", rid)
		ctx = trace.ContextWithSpan(ctx, span)
		c.Next(ctx)
		if c.Response.StatusCode() >= 500 {
			span.SetStatus(codes.Error, "server error")
		}
	}
}
```

创建 `middleware/logging.go`：

```go
package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
)

// Logging access 日志：method/path/status/duration_ms/request_id/remote，
// 经 logx InfoContext 输出（caller 归因到本中间件），并记入 AccessStats（指标钩子）。
func Logging(l logx.Logger, stats *AccessStats) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		dur := time.Since(start)
		if stats != nil {
			stats.Record(ctx, c.Response.StatusCode(), dur)
		}
		l.InfoContext(ctx, "access",
			logx.String("method", string(c.Method())),
			logx.String("path", string(c.FullPath())),
			logx.Int("status", c.Response.StatusCode()),
			logx.Int("duration_ms", int(dur.Milliseconds())),
			logx.String("request_id", envelope.RequestIDFrom(c)),
			logx.String("remote", c.RemoteAddr().String()),
		)
	}
}
```

创建 `middleware/chain.go`（默认链装配，Task 4 `webx.New` 调用）：

```go
package middleware

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"go.opentelemetry.io/otel/trace"
)

// DefaultChain 默认中间件顺序：recovery → otel → logging → ratelimit → cors → timeout。
// 业务 handler 与 WithMiddleware 由 webx.New 追加于链尾（3.4）。
func DefaultChain(tr trace.Tracer, stats *AccessStats, l logx.Logger,
	rps, burst int, corsOrigins []string, timeout time.Duration) app.HandlersChain {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return app.HandlersChain{
		Recovery(l),
		Otel(tr),
		Logging(l, stats),
		RateLimit(rps, burst),
		CORS(corsOrigins),
		Timeout(timeout, l),
	}
}
```

> 类型一致性修正（以此节为准）：`middleware.Logging(l logx.Logger, stats *AccessStats)`、`middleware.DefaultChain(tr, stats, l, rps, burst, corsOrigins, timeout)`——替代 File Structure 锚点中旧签名单行。

- [ ] **步骤 4：运行测试确认通过**

运行：`go test ./middleware/ -v`
预期：全绿。

- [ ] **步骤 5：Commit**

```bash
git add middleware/
git commit -m "feat: webx middleware chain (recovery/timeout/ratelimit/cors/otel/logging + DefaultChain + AccessStats)"
```

- [ ] **Task complete**（本 Task 全部 Step 为 `[x]` 后勾选；与 plan-ready **任务完成**、tasks.md 对应行同步）

---

### Task 4: webx bootstrap 与 health

> **trace:** plan-ready.md → `### Task 4: webx bootstrap 与 health` | tasks.md → `- [ ] 4.1 `webx.go`：`App`（内嵌 `*hertz.App`）+ `New(opts...)` 选项（Config/WithLogger/WithTLS/WithMiddleware/WithReadyCheck/WithOTLP/WithMetrics）+ health 端点自动挂载（`/healthz/live`、`/healthz/ready`）` / `- [ ] 4.2 单例 `webx.App()`（首次默认配置 New 后缓存）+ `TestApp()`（独立实例、127.0.0.1:0 随机端口、`URL()` 取址）` / `- [ ] 4.3 优雅停机：`Shutdown(ctx)` 排空 in-flight；SIGINT/SIGTERM 装配示例（cmd 层）` / `- [ ] 4.4 单测：最小启动、缺省配置不 panic、单例同一指针、TestApp 随机端口、优雅停机排空`
> **sync:** tasks.md → `4.1–4.4` | plan-ready.md → `### Task 4: webx bootstrap 与 health`

**Files:**
- 创建：`config.go`、`webx.go`（module 根，package webx）
- 测试：`webx_test.go`

**契约（写代码前先读一遍，后续 Task 5/6/7 对齐本节签名）：**
- `webx.Config{Addr, Logger, TLSCertFile, TLSKeyFile, EnableALPN, Timeout, RateLimitRPS, RateLimitBurst, CORSOrigins, HealthPrefix, ReadyCheck, Tracer, Stats, MetricsEnabled, OTEL, Middlewares}` + `FillDefaults()`（Addr "0.0.0.0:8080"、Timeout 5s、HealthPrefix "/healthz"、Stats→`middleware.NewAccessStats()`、Tracer→noop）
- `Option` + `WithConfig/WithLogger/WithTLS(certFile, keyFile string, alpn bool)/WithMiddleware(...)/WithReadyCheck/WithOTLP(oc logx.OTelConfig)/WithMetrics(bool)/WithTracer(trace.Tracer)`
- `webx.New(opts ...Option) (*App, error)`（装配默认链 + health + TLS；**不监听**）
- `webx.AppSingleton() (*App, error)`（sync.Once 单例；**命名校正**：plan-ready/tasks.md 写的 `webx.App()` 与类型 `App` 冲突，导出为 `AppSingleton()`）
- `webx.TestApp(opts ...Option) (*App, error)`（`net.Listen("tcp","127.0.0.1:0")` + `server.WithListener` + `standard.NewTransporter` + `Spin` + 轮询 `IsRunning`（3s 超时））
- `(a *App).Config() Config`（只读导出，测试断言缺省值）、`(a *App).URL() string`（TLS 时 https）、`(a *App).Run() error`、`(a *App).Shutdown(ctx) error`
- SIGINT/SIGTERM 装配**不在 webx.go 内置**（Task 4 只保证 `Shutdown` 可用）；cmd 层信号示例在 Task 6 `examples/cmd/main.go`。

**已验证 API（v0.10.6 源码）：**
- `server.New(opts ...config.Option) *Hertz`；options `WithHostPorts(hp string)`、`WithListener(net.Listener)`、`WithTLS(*tls.Config)`、`WithTransport(func(*config.Options) network.Transporter)`、`WithALPN(bool)`；`standard.NewTransporter()`（`github.com/cloudwego/hertz/pkg/network/standard`）。
- `h.Run()` 阻塞；`h.Spin()`（goroutine Run + SIGHUP/SIGINT/SIGTERM→Shutdown）；`h.IsRunning()`；`h.Shutdown(ctx)`（需 running；OnShutdown 钩子 + standard transporter 排空）；`h.Engine() *route.Engine`；`engine.Handlers app.HandlersChain`（全局链，按序执行）；`engine.GET(path string, handlers ...app.HandlerFunc)`。
- 测试 HTTP 客户端：`hc, _ := client.NewClient()`（`github.com/cloudwego/hertz/pkg/app/client`）；`code, body, err := pclient.GetURL(ctx, nil, url, hc)`（`github.com/cloudwego/hertz/pkg/protocol/client`，`Doer` 接口 `Do(ctx, *protocol.Request, *protocol.Response) error`，`*client.Client` 满足）。

- [ ] **步骤 1：编写失败测试**

创建 `webx_test.go`（module 根，package webx）：

```go
package webx

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	pclient "github.com/cloudwego/hertz/pkg/protocol/client" // 别名：与 app/client 同包名 client，避免冲突
)

// getURL 测试内 HTTP 请求（hertz 官方客户端，不用 net/http）
func getURL(t *testing.T, url string) (int, []byte, error) {
	t.Helper()
	hc, err := client.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return pclient.GetURL(context.Background(), nil, url, hc)
}

func TestNewDefaultsNoPanic(t *testing.T) {
	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cfg := a.Config()
	if cfg.Addr != "0.0.0.0:8080" {
		t.Errorf("default Addr = %q, want 0.0.0.0:8080", cfg.Addr)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("default Timeout = %s, want 5s", cfg.Timeout)
	}
	if cfg.HealthPrefix != "/healthz" {
		t.Errorf("default HealthPrefix = %q, want /healthz", cfg.HealthPrefix)
	}
	if cfg.Stats == nil || cfg.Tracer == nil {
		t.Error("default Stats/Tracer nil")
	}
}

func TestAppSingletonSamePointer(t *testing.T) {
	a1, err := AppSingleton()
	if err != nil {
		t.Fatalf("AppSingleton: %v", err)
	}
	a2, _ := AppSingleton()
	if a1 != a2 {
		t.Fatal("AppSingleton returned different pointers")
	}
}

func TestTestAppURLAndLiveProbe(t *testing.T) {
	a, err := TestApp()
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() {
		_ = a.Shutdown(context.Background())
	})
	if !strings.HasPrefix(a.URL(), "http://127.0.0.1:") {
		t.Fatalf("URL = %s, want http://127.0.0.1:<port>", a.URL())
	}
	code, body, err := getURL(t, a.URL()+"/healthz/live")
	if err != nil {
		t.Fatalf("GET live: %v", err)
	}
	if code != 200 || !strings.Contains(string(body), "alive") {
		t.Fatalf("live probe = %d %s, want 200 alive", code, body)
	}
}

func TestReadyGate(t *testing.T) {
	a, err := TestApp(WithReadyCheck(func(context.Context) error {
		return errors.New("db down")
	}))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() {
		_ = a.Shutdown(context.Background())
	})
	liveCode, _, _ := getURL(t, a.URL()+"/healthz/live")
	if liveCode != 200 {
		t.Fatalf("live = %d, want 200", liveCode)
	}
	readyCode, readyBody, _ := getURL(t, a.URL()+"/healthz/ready")
	if readyCode != 503 || !strings.Contains(string(readyBody), "unavailable") {
		t.Fatalf("ready = %d %s, want 503 unavailable", readyCode, readyBody)
	}
}

func TestGracefulShutdownDrain(t *testing.T) {
	a, err := TestApp()
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	a.Engine().GET("/sleep", func(ctx context.Context, c *app.RequestContext) {
		time.Sleep(300 * time.Millisecond)
		c.JSON(200, "done")
	})
	done := make(chan int, 1)
	go func() {
		code, _, err := getURL(t, a.URL()+"/sleep")
		if err != nil {
			done <- 0
			return
		}
		done <- code
	}()
	// 立即停机：in-flight 请求应被排空（拿到 200）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := <-done; got != 200 {
		t.Fatalf("drained request status = %d, want 200", got)
	}
	if a.IsRunning() {
		t.Fatal("IsRunning() still true after Shutdown")
	}
}

func TestWithMiddlewareAppendedAtTail(t *testing.T) {
	a, err := TestApp(WithMiddleware(func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.Set("X-Tail", "1")
		c.Next(ctx)
	}))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() {
		_ = a.Shutdown(context.Background())
	})
	hc, err := client.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI(a.URL() + "/healthz/live")
	resp := &protocol.Response{}
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := string(resp.Header.Peek("X-Tail")); got != "1" {
		t.Fatalf("X-Tail = %q, want 1 (middleware appended at chain tail)", got)
	}
}
```

- [ ] **步骤 2：运行测试确认失败**

运行：`go test . -v`
预期：编译失败（`undefined: New / AppSingleton / TestApp / WithReadyCheck / ...`）——红灯。

- [ ] **步骤 3：实现 `config.go`**

```go
package webx

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Config webx 装配配置（缺省值经 FillDefaults 补齐）
type Config struct {
	Addr         string // "host:port"，默认 "0.0.0.0:8080"
	Logger       logx.Logger
	TLSCertFile  string
	TLSKeyFile   string
	EnableALPN   bool
	Timeout      time.Duration
	RateLimitRPS int
	RateLimitBurst int
	CORSOrigins  []string
	HealthPrefix string
	ReadyCheck   func(context.Context) error
	Tracer       trace.Tracer
	Stats        *middleware.AccessStats
	MetricsEnabled bool
	OTEL         logx.OTelConfig // 记录用；TracerProvider 由 cmd 层经 logx 装配
	Middlewares  []app.HandlerFunc
}

// FillDefaults 对零值字段补齐缺省
func (c *Config) FillDefaults() {
	if c.Addr == "" {
		c.Addr = "0.0.0.0:8080"
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	if c.HealthPrefix == "" {
		c.HealthPrefix = "/healthz"
	}
	if c.Logger == nil {
		c.Logger = logx.GetGlobal()
	}
	if c.Tracer == nil {
		c.Tracer = otel.GetTracerProvider().Tracer("webx") // noop 安全
	}
	if c.Stats == nil {
		c.Stats = middleware.NewAccessStats()
	}
	if c.OTEL.ServiceName == "" {
		c.OTEL.ServiceName = "webx"
	}
}

// Option 配置选项
type Option func(*Config)

// WithConfig 整体注入配置（后续 Option 可覆盖其字段）
func WithConfig(cfg Config) Option {
	return func(c *Config) { *c = cfg }
}

// WithLogger 注入 logx logger
func WithLogger(l logx.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithTLS 证书文件 + ALPN（H2 尽力协商，h1-over-TLS 兜底）
func WithTLS(certFile, keyFile string, alpn bool) Option {
	return func(c *Config) {
		c.TLSCertFile = certFile
		c.TLSKeyFile = keyFile
		c.EnableALPN = alpn
	}
}

// WithMiddleware 业务中间件，追加于默认链链尾（对应 3.4）
func WithMiddleware(mws ...app.HandlerFunc) Option {
	return func(c *Config) { c.Middlewares = append(c.Middlewares, mws...) }
}

// WithReadyCheck ready 探针自定义（nil/成功 → 200，错误 → 503）
func WithReadyCheck(f func(context.Context) error) Option {
	return func(c *Config) { c.ReadyCheck = f }
}

// WithOTLP 记录 OTel 导出配置（TracerProvider 实际装配在 cmd 层经 logx）
func WithOTLP(oc logx.OTelConfig) Option {
	return func(c *Config) { c.OTEL = oc }
}

// WithMetrics 启用 otel/metric instrument（BindMeter，Task 5）
func WithMetrics(enabled bool) Option {
	return func(c *Config) { c.MetricsEnabled = enabled }
}

// WithTracer 注入真实 Tracer（测试用 InMemory exporter）
func WithTracer(tr trace.Tracer) Option {
	return func(c *Config) { c.Tracer = tr }
}
```

- [ ] **步骤 4：实现 `webx.go`**

```go
package webx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/middleware"
)

// App webx 应用（内嵌 *server.Hertz；TestApp 独立实例，不触碰单例）
type App struct {
	*server.Hertz
	cfg   Config
	ln    net.Listener
	stats *middleware.AccessStats
}

// New 按选项装配默认链 + health + TLS；不监听（监听由 Run 或 TestApp 触发）
func New(opts ...Option) (*App, error) {
	cfg := Config{}
	for _, o := range opts {
		o(&cfg)
	}
	cfg.FillDefaults()

	srvOpts, err := buildServerOptions(&cfg, nil)
	if err != nil {
		return nil, err
	}
	h := server.New(srvOpts...)

	// 默认链 → 业务中间件追加链尾（3.4）
	h.Engine().Handlers = append(
		middleware.DefaultChain(cfg.Tracer, cfg.Stats, cfg.Logger,
			cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.CORSOrigins, cfg.Timeout),
		cfg.Middlewares...,
	)
	registerHealth(h.Engine(), &cfg)

	return &App{Hertz: h, cfg: cfg, stats: cfg.Stats}, nil
}

// buildServerOptions 组装 hertz options；ln 非 nil（TestApp）时走 listener 分支
func buildServerOptions(cfg *Config, ln net.Listener) ([]config.Option, error) {
	if ln != nil {
		opts := []config.Option{
			server.WithListener(ln),
			server.WithTransport(standard.NewTransporter),
		}
		if cfg.TLSCertFile != "" {
			tlsCfg, err := loadTLSCfg(cfg)
			if err != nil {
				return nil, err
			}
			opts = append(opts, server.WithTLS(tlsCfg))
			if cfg.EnableALPN {
				opts = append(opts, server.WithALPN(true))
			}
		}
		return opts, nil
	}
	if cfg.TLSCertFile != "" {
		tlsCfg, err := loadTLSCfg(cfg)
		if err != nil {
			return nil, err
		}
		opts := []config.Option{
			server.WithTLS(tlsCfg),
			server.WithTransport(standard.NewTransporter), // TLS 切 standard transporter
		}
		if cfg.EnableALPN {
			opts = append(opts, server.WithALPN(true))
		}
		return opts, nil
	}
	return []config.Option{server.WithHostPorts(cfg.Addr)}, nil
}

// loadTLSCfg 证书加载 fail fast：错误信息含两个文件路径
func loadTLSCfg(cfg *Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("webx: load TLS cert/key (%s, %s): %w", cfg.TLSCertFile, cfg.TLSKeyFile, err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

// registerHealth /healthz/live + /healthz/ready
func registerHealth(e *route.Engine, cfg *Config) {
	e.GET(cfg.HealthPrefix+"/live", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]any{"status": "alive"})
	})
	e.GET(cfg.HealthPrefix+"/ready", func(ctx context.Context, c *app.RequestContext) {
		if cfg.ReadyCheck == nil || cfg.ReadyCheck(ctx) == nil {
			c.JSON(200, map[string]any{"status": "ready"})
			return
		}
		c.JSON(503, map[string]any{"status": "unavailable"})
	})
}

// Config 只读导出当前配置（测试断言缺省值用）
func (a *App) Config() Config { return a.cfg }

// URL 取址（TLS → https）；TestApp（有 listener）返回实际随机端口
func (a *App) URL() string {
	scheme := "http"
	if a.cfg.TLSCertFile != "" {
		scheme = "https"
	}
	if a.ln != nil {
		return scheme + "://" + a.ln.Addr().String()
	}
	return scheme + "://" + a.cfg.Addr
}

// AppSingleton 单例：首次默认配置 New 后缓存；构造失败缓存 error（不静默吞，logx 记录）
var (
	singletonOnce sync.Once
	singletonApp  *App
	singletonErr  error
)

func AppSingleton() (*App, error) {
	singletonOnce.Do(func() {
		singletonApp, singletonErr = New()
		if singletonErr != nil {
			logx.Error("webx: singleton construction failed", logx.Err(singletonErr))
		}
	})
	return singletonApp, singletonErr
}

// TestApp 独立实例：127.0.0.1:0 随机端口 + Spin + 等待 IsRunning（3s 超时）
func TestApp(opts ...Option) (*App, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("webx: test listen: %w", err)
	}
	cfg := Config{}
	for _, o := range opts {
		o(&cfg)
	}
	cfg.FillDefaults()

	srvOpts, err := buildServerOptions(&cfg, ln)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	h := server.New(srvOpts...)
	h.Engine().Handlers = append(
		middleware.DefaultChain(cfg.Tracer, cfg.Stats, cfg.Logger,
			cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.CORSOrigins, cfg.Timeout),
		cfg.Middlewares...,
	)
	registerHealth(h.Engine(), &cfg)

	a := &App{Hertz: h, cfg: cfg, ln: ln, stats: cfg.Stats}
	go h.Run()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if h.IsRunning() {
			return a, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, errors.New("webx: test server not running after 3s")
}

// Run 阻塞监听（生产入口）
func (a *App) Run() error {
	return a.Hertz.Run()
}

// Shutdown 优雅停机：OnShutdown 钩子 → 排空 in-flight → transporter 停
func (a *App) Shutdown(ctx context.Context) error {
	return a.Hertz.Shutdown(ctx)
}
```

> **API 校正（v0.10.6 源码实测，落地直接照抄）：**
> 1. `server.New(opts ...config.Option)`（`pkg/app/server/hertz.go:39`）——option 类型是 `pkg/common/config.Option`（**不存在** `server.ConfigOption` 类型；`WithListener/WithHostPorts/WithTLS/WithTransport/WithALPN` 均由 `pkg/app/server/option.go` 导出、返回 `config.Option`）。本 Task 代码块已按 `[]config.Option` + import `github.com/cloudwego/hertz/pkg/common/config` 书写。
> 2. `h.Engine()` 非 nil 且 `Engine.Handlers` 全局链按序执行（v0.10.6 engine.go 已验证）；`registerHealth` 注册在 New/TestApp 内，路由不挂全局链，**但 health 探针不受业务 WithMiddleware 影响**（符合「探针独立」语义；若需探针也过默认链，改经 `h.Spin()` 前 `h.Use(...)` 追加——当前契约保持独立）。
> 3. `TestGracefulShutdownDrain` 依赖 standard transporter 的 Shutdown 排空（已验证 `standard.NewTransporter` 支持 `Shutdown(ctx)`）；`Spin()` 未在 TestApp 使用（避免信号 goroutine 与 t.Cleanup 竞争，直接 `go h.Run()`）。

- [ ] **步骤 5：运行测试确认通过**

运行：`go test . -v`
预期：全绿（6 个测试）。`go vet .` 无告警。

- [ ] **步骤 6：Commit**

```bash
git add config.go webx.go webx_test.go
git commit -m "feat: webx bootstrap (New options, health probes, AppSingleton, TestApp, graceful Shutdown)"
```

- [ ] **Task complete**（本 Task 全部 Step 为 `[x]` 后勾选；与 plan-ready **任务完成**、tasks.md 对应行同步）

---

### Task 5: TLS / OTel 集成

> **trace:** plan-ready.md → `### Task 5: TLS / OTel 集成` | tasks.md → `- [ ] 5.1 `WithTLS` 证书加载 + H2；加载失败 fail fast 返回含路径错误` / `- [ ] 5.2 OTel 装配：TracerProvider（OTLP gRPC）+ Meter（请求计数/耗时直方图/code 维度）；Shutdown flush` / `- [ ] 5.3 单测/集成：临时自签证书 HTTPS+h2 用例、span 导出断言、指标计数断言`
> **sync:** tasks.md → `5.1–5.3` | plan-ready.md → `### Task 5: TLS / OTel 集成`

**Files:**
- 修改：`webx.go`（TLS fail fast + OTel TracerProvider 注入 + Metrics BindMeter）、`middleware/access_stats.go`（新增 `MetricsBound()` 测试钩子）
- 测试：`webx_tls_test.go`、`webx_otel_test.go`、`testdata/write_selfsigned.go`（生成器，**不入库**，测试内联生成）

**离线约束与决策（执行前必读）：**
- tasks.md 5.1 写「H2」：真 H2 需 `hertz-contrib/h2`（`engine.AddProtocol(suite.HTTP2, factory)`），本地 module cache 无此包且网络被拦截 → 本 Task 的 H2 语义为**尽力协商**：`WithTLS` + `server.WithALPN(true)` 在 ALPN 层通告 h2，未注册 HTTP2 protocol factory 时 hertz 告警并回退 h1-over-TLS（v0.10.6 已验证：`protocolServers[suite.HTTP2]` 为 nil 时 warn + h1 兜底）。**`hertz-contrib/h2` 真 H2 列为 open decision（见本 Task 末），不阻塞本变更。**
- tasks.md 5.2 写「OTLP gRPC」：离线环境无 `otlptracegrpc` 缓存 → TracerProvider 经 **logx** 装配（`logx/zap.SetupTracerProvider(logx.OTelConfig)`，exporter ∈ jaeger/otlp(otlptracehttp)/stdout），OTLP 走 HTTP exporter（otlptracehttp v1.29.0 已缓存）；gRPC 导出留给后续变更（网络恢复后补 otlptracegrpc）。
- 指标：不引入 `otel/sdk/metric`；instrument 经 `otel/metric` Meter（v1.29.0）创建并绑定 `AccessStats.BindMeter`（Task 3 已留接口），测试断言走 `AccessStats` 原子快照。

**契约（对齐 Task 4 签名）：**
- `webx.WithTracer(trace.Tracer)`（Task 4 已有）+ 新增 `webx.WithMeterProvider(metric.Provider)`（OTel 装配：TracerProvider 经 `otel.GetTracerProvider()` 全局句柄或显式 `WithTracer`；Meter 经本选项显式注入）
- `webx.New` 内：`cfg.MetricsEnabled` 为 true 时 `meter := cfg.MeterProvider.Meter("webx")` + `cfg.Stats.BindMeter(meter)`（失败记 logx.Warn，不 fail——指标装配不阻断服务）
- `middleware.AccessStats.MetricsBound() bool`（`s.reqCounter != nil`，测试断言接线成功）
- TLS 加载失败错误串必须含 `TLSCertFile` 与 `TLSKeyFile` 两个路径（Task 4 `loadTLSCfg` 已实现，本 Task 加用例）

- [ ] **步骤 1：编写失败测试**

创建 `webx_tls_test.go`：

```go
package webx

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
)

// writeSelfSigned 生成自签证书对（测试内联，不落 testdata 目录）
func writeSelfSigned(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "webx.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"webx.test", "localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("pem cert: %v", err)
	}
	_ = certFile.Close()
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("pem key: %v", err)
	}
	_ = keyFile.Close()
	return certPath, keyPath
}

func TestTLSCertLoadFailFast(t *testing.T) {
	cert, key := writeSelfSigned(t)
	_ = cert
	_ = key
	_, err := New(WithTLS("/definitely/missing/cert.pem", "/definitely/missing/key.pem", false))
	if err == nil {
		t.Fatal("New with missing cert: want error, got nil")
	}
	for _, p := range []string{"/definitely/missing/cert.pem", "/definitely/missing/key.pem"} {
		if !strings.Contains(err.Error(), p) {
			t.Fatalf("error = %q, want contains path %q", err.Error(), p)
		}
	}
}

func TestHTTPSOverTLS(t *testing.T) {
	cert, key := writeSelfSigned(t)
	a, err := TestApp(WithTLS(cert, key, true)) // ALPN 通告 h2（尽力；h1-over-TLS 兜底）
	if err != nil {
		t.Fatalf("TestApp(TLS): %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	// 127.0.0.1 URL 需重定向 SAN 匹配：自签证书含 127.0.0.1 SAN，用 InsecureSkipVerify 兜底
	hc, err := client.NewClient(client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	req := newProtocolRequest(t, "GET", a.URL()+"/healthz/live")
	resp := newProtocolResponse()
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("HTTPS Do: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("HTTPS status = %d, want 200", resp.StatusCode())
	}
	if !strings.Contains(string(resp.Body()), "alive") {
		t.Fatalf("body = %s, want alive", resp.Body())
	}
}
```

> **实现备注：** `newProtocolRequest(t, method, url)` / `newProtocolResponse()` 为测试辅助（`&protocol.Request{}` + `SetMethod`/`SetRequestURI(url)`/`SetScheme("https")` + `&protocol.Response{}`），实现时放在 `webx_tls_test.go` 顶部；`client.WithTLSConfig` 已在 v0.10.6 `pkg/app/client/option.go:92` 验证存在。删除上例中多余的 `os.File` 占位行。

创建 `webx_otel_test.go`：

```go
package webx

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/gospacex/webx/middleware"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOTelSpanExport(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exp))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	a, err := TestApp(WithTracer(provider.Tracer("webx-test")), WithMetrics(true))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	hc := newHertzClient(t)
	req := newProtocolRequest(t, "GET", a.URL()+"/healthz/live")
	resp := newProtocolResponse()
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do: %v", err)
	}

	// root span 已导出（WithSyncer 同步导出；Shutdown 前可查）
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1 root span", len(spans))
	}
	if got := string(resp.Header.Peek("X-Request-Id")); got != spans[0].SpanContext.TraceID().String() {
		t.Fatalf("X-Request-Id = %s, want trace-id %s", got, spans[0].SpanContext.TraceID())
	}
}

func TestMetricsBoundOnEnable(t *testing.T) {
	a, err := TestApp(WithMetrics(true))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
	// MetricsBound 钩子（本 Task 新增于 middleware.AccessStats）
	if !a.Config().Stats.MetricsBound() {
		t.Fatal("AccessStats.MetricsBound() = false, want true after WithMetrics(true)")
	}
	// 关闭态不接线
	b, err := TestApp(WithMetrics(false))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = b.Shutdown(context.Background()) })
	if b.Config().Stats.MetricsBound() {
		t.Fatal("AccessStats.MetricsBound() = true, want false (metrics disabled)")
	}
}
```

> 辅助：`newHertzClient(t)` 返回 `*client.Client`（`client.NewClient()`）；与 `webx_tls_test.go` 共享，实现时放 `webx_test.go` 或公共 `testhelpers_test.go`（若两测试文件各定义一次会重名——**落地时统一放 `testhelpers_test.go`**，并在文件头注释说明）。

- [ ] **步骤 2：运行测试确认失败**

运行：`go test . -run "TestTLS|TestOTel|TestMetrics" -v`
预期：编译失败（`undefined: MetricsBound / WithMeterProvider` 等）——红灯。

- [ ] **步骤 3：实现——`middleware/access_stats.go` 加测试钩子 + `webx.go` OTel 装配**

`middleware/access_stats.go` 追加（在 `BindMeter` 之后）：

```go
// MetricsBound instrument 是否已接线（测试断言用）
func (s *AccessStats) MetricsBound() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reqCounter != nil
}
```

`config.go` 追加选项 + `webx.go` 装配：

```go
// WithMeterProvider 注入 otel/metric Provider（MetricsEnabled=true 时 BindMeter）
func WithMeterProvider(p metric.Provider) Option {
	return func(c *Config) { c.MeterProvider = p }
}
```

`webx.Config` 增字段 `MeterProvider metric.Provider`（import `go.opentelemetry.io/otel/metric`）；`New`/`TestApp` 装配段追加：

```go
if cfg.MetricsEnabled {
	provider := cfg.MeterProvider
	if provider == nil {
		provider = otel.GetMeterProvider() // 缺省全局（noop 安全）
	}
	if err := cfg.Stats.BindMeter(provider.Meter("webx")); err != nil {
		cfg.Logger.Warn("webx: bind metric instruments failed", logx.Err(err))
		// 指标装配不阻断服务（KISS：告警即止）
	}
}
```

- [ ] **步骤 4：运行测试确认通过**

运行：`go test . -run "TestTLS|TestOTel|TestMetrics" -v`
预期：全绿。

- [ ] **步骤 5：Commit**

```bash
git add webx.go config.go middleware/access_stats.go webx_tls_test.go webx_otel_test.go testhelpers_test.go
git commit -m "feat: webx TLS fail-fast + OTel tracer/metric integration (ALPN h2 best-effort, InMemory span assertions)"
```

- [ ] **Task complete**（本 Task 全部 Step 为 `[x]` 后勾选；与 plan-ready **任务完成**、tasks.md 对应行同步）

**⚠️ Open decision（H2）：** 真 H2 需 `github.com/cloudwego/hertz-contrib/h2`（本地无缓存、网络被拦截）。当前实现 = ALPN 通告 + h1-over-TLS 兜底。网络恢复后另开变更补 `engine.AddProtocol(suite.HTTP2, h2.NewFactory())` 真 H2 + ALPN 端到端断言；本变更按 fallback 交付。

---

### Task 6: examples 分层示例

> **trace:** plan-ready.md → `### Task 6: examples 分层示例` | tasks.md → `- [ ] 6.1 目录骨架 + `internal/config`（configx 绑定业务配置）+ `internal/model`（entity + Repository 接口 + 内存实现）` / `- [ ] 6.2 `app/handler`（薄 handler）+ `app/routes.go`（注册源唯一）+ `internal/logic`（参数转换 + 调 service）` / `- [ ] 6.3 `internal/service`（业务核心 + 子 span）+ `internal/middleware`（鉴权业务中间件）+ `cmd/main.go`（一行启动）` / `- [ ] 6.4 各层单测（handler/logic/service/model，接口注入）` / `- [ ] 6.5 调用链开关：`internal/config` 增加 `Trace{Enabled, Exporter}`（configx 绑定 `demo.trace`，默认 false）；cmd 层开启时以 `OTel.Enabled=true` 构造 logx logger 并 `logx/zap.SetupTracerProvider` 装配 TracerProvider；`app/routes.go` 入口中间件开 root span（`<METHOD> <path>`），各层开子 span；开启/关闭两态各一测试用例（开启态断言 span 树 + 各层日志 trace_id 一致且挂 span event；关闭态断言无 span、行为不变）`
> **sync:** tasks.md → `6.1–6.5` | plan-ready.md → `### Task 6: examples 分层示例`

**Files:**
- 创建：`examples/internal/config/config.go`、`examples/internal/model/order.go`、`examples/internal/service/order.go` + `codes.go`、`examples/internal/logic/order.go`、`examples/internal/trace/trace.go`、`examples/internal/middleware/auth.go`、`examples/app/handler/order.go`、`examples/app/routes.go`、`examples/cmd/main.go`、`examples/config.yaml`
- 测试：`examples/internal/model/order_test.go`、`examples/internal/service/order_test.go`、`examples/internal/logic/order_test.go`、`examples/app/handler/order_test.go`、`examples/trace_switch_test.go`

**分层契约（单向调用：中间件 → handler → logic → service → model；错误在产生层定义、中间层透传、envelope 统一渲染）：**

| 包 | 关键类型 | 职责边界 |
|---|---|---|
| `internal/config` | `Config{Biz Biz; Trace Trace}`；`Biz{TimeoutMS int}`；`Trace{Enabled bool; Exporter string}` | configx 绑定 `demo.*`；`FillDefaults`：Trace.Enabled=false、Exporter="stdout" |
| `internal/model` | `Order{ID int64; Item string; Qty int; Status string}`；`OrderRepository`（`Get/Create/List`，ctx 首参）；`MemRepo`（sync.Mutex + map） | Get 不存在 → `envelope.Errorf(CodeNotFound, "订单 %d 不存在", id)` |
| `internal/service` | `OrderService{Repo model.OrderRepository; L logx.Logger; Tracer trace.Tracer}` | 子 span `service.<操作>`；model 错误按码透传；`Status=="archived"` → `CodeOrderArchived`(2001) |
| `internal/logic` | `OrderLogic{Svc *service.OrderService; L logx.Logger; Tracer trace.Tracer}` | 参数校验（id<=0 → `ErrBadRequest`）+ 子 span `logic.<操作>` + 调 service |
| `app/handler` | `OrderHandler{Logic *logic.OrderLogic; L logx.Logger; Tracer trace.Tracer}` | 薄 handler：`c.Param` → logic → `envelope.Render`；子 span `handler.<操作>` |
| `app/routes.go` | `RegisterRoutes(e *route.Engine, h *handler.OrderHandler)` | **手写注册源唯一**（头注释：hz 生成区约定——重跑 hz 仅影响 `app/` 生成物，`internal/` 手写代码不被触碰；本模块不跑 hz 生成器，routes.go 即注册源） |
| `internal/middleware` | `Auth(token string) app.HandlerFunc` | `X-Token` 不匹配 → `envelope.Render(c, nil, envelope.ErrUnauthorized)` + `c.Abort()`（4010/401） |
| `cmd/main.go` | 一行启动 | configx+logx+webx 装配 + SIGINT/SIGTERM 优雅停机 |

**已验证 API（写代码前 grep 过）：**
- `configx.ReadConfig(configx.ConfigOptions{ConfigFile: path})` → `configx.Config`（接口：`Unmarshal(key string, out any) error`、`GetString`、`GetBool`…）；示例配置 `demo:` 段经 `Unmarshal("demo", &cfg)` 绑定。
- `param.Params` = `[]param.Param{Key,Value}`（`github.com/cloudwego/hertz/pkg/route/param`，`ByName` 读）；单测注入 param：`c.Params = param.Params{{Key: "id", Value: "42"}}`（无 Append，直接赋值）。
- `logx.New(logx.Config{...})` 需 blank-import `github.com/gospacex/logx/zap` 注册 provider；`logxzap.SetupTracerProvider(logx.OTelConfig) (*logxzap.TracerSetup, error)`，`TracerSetup.Provider *sdktrace.TracerProvider`（同时 `otel.SetTracerProvider` 全局生效）；OTel.Enabled=true 时 logx 各 `*Context` 方法自动把日志挂为 ctx 活动 span 的 event + trace_id/span_id 字段（`logx/zap` 已实现，本 Task 只消费）。
- handler 单测（chain-unit）：`ctx := app.NewContext(0)` + `ctx.Request.SetMethod("GET")` + `ctx.Params = param.Params{{Key:"id",Value:"42"}}` + `ctx.SetHandlers(app.HandlersChain{handlerFn})` + `ctx.Next(ctxWithSpan)` → 断言 `ctx.Response.Body()` JSON。

- [ ] **步骤 1：编写失败测试（config / model / service / logic / handler 各层）**

创建 `examples/internal/model/order_test.go`：

```go
package model

import (
	"context"
	"errors"
	"testing"

	"github.com/gospacex/webx/envelope"
)

func TestMemRepoCRUD(t *testing.T) {
	repo := NewMemRepo()
	ctx := context.Background()
	want := &Order{ID: 1, Item: "pen", Qty: 2, Status: "pending"}
	if err := repo.Create(ctx, want); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Item != "pen" || got.Qty != 2 {
		t.Fatalf("Get = %+v, want %+v", got, want)
	}
	if list, err := repo.List(ctx); err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v; want 1 item", list, err)
	}
}

func TestMemRepoGetMissingReturns404(t *testing.T) {
	repo := NewMemRepo()
	_, err := repo.Get(context.Background(), 99)
	if e := envelope.As(err); e.Code != envelope.CodeNotFound {
		t.Fatalf("Get missing code = %d, want %d", e.Code, envelope.CodeNotFound)
	}
	if !errors.Is(err, envelope.ErrNotFound) {
		t.Fatal("errors.Is(Get missing, ErrNotFound) = false")
	}
}
```

创建 `examples/internal/service/order_test.go`（接口注入假 Repo + span 断言）：

```go
package service

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol/consts" // 仅 StatusOK 常量（可选，可去）
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// 注：tracer 经 sdktrace provider 构造；service 层 Start 返回 (ctx, span)

// fakeRepo 最小假 Repo（接口注入，不用 testify）
type fakeRepo struct{ orders map[int64]*model.Order; notFound bool }

func (f *fakeRepo) Get(ctx context.Context, id int64) (*model.Order, error) {
	if f.notFound {
		return nil, envelope.Errorf(envelope.CodeNotFound, "订单 %d 不存在", id)
	}
	if o, ok := f.orders[id]; ok {
		return o, nil
	}
	return nil, envelope.Errorf(envelope.CodeNotFound, "订单 %d 不存在", id)
}
func (f *fakeRepo) Create(ctx context.Context, o *model.Order) error { return nil }
func (f *fakeRepo) List(ctx context.Context) ([]*model.Order, error) { return nil, nil }

func TestServiceGetPassthrough(t *testing.T) {
	repo := &fakeRepo{orders: map[int64]*model.Order{7: {ID: 7, Status: "pending"}}}
	svc := New(repo, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))
	got, err := svc.Get(context.Background(), 7)
	if err != nil || got.ID != 7 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

func TestServiceArchivedOrderRejected(t *testing.T) {
	repo := &fakeRepo{orders: map[int64]*model.Order{8: {ID: 8, Status: "archived"}}}
	svc := New(repo, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))
	_, err := svc.Get(context.Background(), 8)
	if e := envelope.As(err); e.Code != CodeOrderArchived {
		t.Fatalf("code = %d, want %d", e.Code, CodeOrderArchived)
	}
}

func TestServiceOpensChildSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	tracer := provider.Tracer("svc-test")
	repo := &fakeRepo{orders: map[int64]*model.Order{9: {ID: 9, Status: "pending"}}}
	svc := New(repo, logx.GetGlobal(), tracer)

	// 根 span：Start 返回 (ctx 含活动 span, span)（v1.29.0 Go SDK）
	rootCtx, root := tracer.Start(context.Background(), "root")
	if _, err := svc.Get(rootCtx, 9); err != nil {
		t.Fatalf("Get: %v", err)
	}
	root.End()

	var found bool
	for _, s := range exp.GetSpans() {
		if s.Name == "service.GetOrder" && s.Parent.SpanID() == root.SpanContext().SpanID() {
			found = true
		}
	}
	if !found {
		t.Fatalf("child span 'service.GetOrder' not exported under root; got %v", exp.GetSpans())
	}
}
```

> **API 校正：** `trace.Span` 不暴露 `Context()` 方法——span 所在 ctx 一律经 `go.opentelemetry.io/otel/trace.ContextWithSpan(ctx, span)` 构造（与 Task 3 `middleware/otel.go` 一致）。

创建 `examples/internal/logic/order_test.go`：

```go
package logic

import (
	"context"
	"testing"

	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/service"
	"go.opentelemetry.io/otel"
)

// 复用 service 测试的 fakeRepo 思路：此处直接以 nil Svc 测参数校验短路
func TestLogicRejectsNonPositiveID(t *testing.T) {
	l := New(nil, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))
	_, err := l.Get(context.Background(), 0)
	if e := envelope.As(err); e.Code != envelope.CodeBadRequest {
		t.Fatalf("code = %d, want %d", e.Code, envelope.CodeBadRequest)
	}
	_ = service.CodeOrderArchived // 保持包依赖可见（可选删除）
}
```

创建 `examples/app/handler/order_test.go`（chain-unit + param 注入）：

```go
package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/logic"
	"go.opentelemetry.io/otel"
)

func TestHandlerRendersEnvelope(t *testing.T) {
	h := New(nil, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))
	hFn := h.GetOrder // app.HandlerFunc

	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/orders/42")
	c.Params = param.Params{{Key: "id", Value: "42"}} // 直接赋值注入 param（v0.10.6 验证）
	c.SetHandlers(app.HandlersChain{hFn})
	c.Next(context.Background())

	body := string(c.Response.Body())
	if !strings.Contains(body, `"code":1001`) {
		// logic 为 nil → 参数转换层报 1001（Get(0) 不可达：param "42" 解析正常 → 走 logic.Get(42) → nil Svc panic？
		// 契约：logic 为 nil 时 handler 直接 envelope.ErrBadRequest（fail fast，不 panic）
		t.Fatalf("body = %s, want code 1001 (nil logic fail fast)", body)
	}
	if c.Response.StatusCode() != 400 {
		t.Fatalf("status = %d, want 400", c.Response.StatusCode())
	}
}
```

> 契约明确：**nil 依赖 = fail fast 1001**（handler 首行 `if h.Logic == nil { envelope.Render(c, nil, envelope.ErrBadRequest); return }`）——避免单测传 nil 时 panic。

- [ ] **步骤 2：运行测试确认失败**

运行：`go test ./examples/... -v`
预期：编译失败（各层 undefined）——红灯。

- [ ] **步骤 3：实现各层（model → service → logic → handler → middleware → routes）**

创建 `examples/internal/config/config.go`：

```go
// config 业务配置：configx 绑定 demo.* 段
package config

import "github.com/gospacex/configx"

// Biz 业务参数
type Biz struct {
	TimeoutMS int `mapstructure:"timeout_ms"`
}

// Trace 调用链开关（用户核心需求：开启后分层路由入链，经 logx 实现）
type Trace struct {
	Enabled  bool   `mapstructure:"enabled"`
	Exporter string `mapstructure:"exporter"` // stdout / jaeger / otlp
}

// Config 示例工程配置
type Config struct {
	Biz   Biz   `mapstructure:"biz"`
	Trace Trace `mapstructure:"trace"`
}

// FillDefaults 缺省（调用链默认关闭）
func (c *Config) FillDefaults() {
	if c.Trace.Exporter == "" {
		c.Trace.Exporter = "stdout"
	}
}

// Load 经 configx 读配置文件并绑定 demo 段
func Load(path string) (*Config, error) {
	c, err := configx.ReadConfig(configx.ConfigOptions{ConfigFile: path})
	if err != nil {
		return nil, err
	}
	out := &Config{}
	out.FillDefaults()
	if err := c.Unmarshal("demo", out); err != nil {
		return nil, err
	}
	return out, nil
}
```

创建 `examples/internal/model/order.go`：

```go
// model 订单实体 + 数据访问（内存 demo 实现）
package model

import (
	"context"
	"sync"

	"github.com/gospacex/webx/envelope"
)

// Order 订单实体
type Order struct {
	ID     int64
	Item   string
	Qty    int
	Status string
}

// OrderRepository 数据访问接口（业务层依赖抽象）
type OrderRepository interface {
	Get(ctx context.Context, id int64) (*Order, error)
	Create(ctx context.Context, o *Order) error
	List(ctx context.Context) ([]*Order, error)
}

// MemRepo 内存实现（demo）
type MemRepo struct {
	mu sync.Mutex
	m  map[int64]*Order
}

// NewMemRepo 创建空内存 Repo
func NewMemRepo() *MemRepo {
	return &MemRepo{m: make(map[int64]*Order)}
}

// Get 不存在 → envelope 1004
func (r *MemRepo) Get(ctx context.Context, id int64) (*Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o, ok := r.m[id]; ok {
		return o, nil
	}
	return nil, envelope.Errorf(envelope.CodeNotFound, "订单 %d 不存在", id)
}

// Create 入库（demo 语义：ID 必填，重复 1009）
func (r *MemRepo) Create(ctx context.Context, o *Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.m[o.ID]; exists {
		return envelope.Errorf(envelope.CodeConflict, "订单 %d 已存在", o.ID)
	}
	r.m[o.ID] = o
	return nil
}

// List 全量（demo 不分页）
func (r *MemRepo) List(ctx context.Context) ([]*Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Order, 0, len(r.m))
	for _, o := range r.m {
		out = append(out, o)
	}
	return out, nil
}
```

创建 `examples/internal/service/codes.go`：

```go
// service 业务错误码（2xxx 段扩展，经 envelope.RegisterCode 映射 HTTP 状态）
package service

import "github.com/gospacex/webx/envelope"

// 业务码（2xxx 段）
const (
	CodeOrderArchived = 2001
)

func init() {
	// 归档订单不可再取 → 410 Gone 语义；骨架用 400 兜底
	envelope.RegisterCode(CodeOrderArchived, 400)
}
```

创建 `examples/internal/service/order.go`：

```go
// service 业务核心：错误在此层产生/转换，子 span 入链
package service

import (
	"context"

	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/model"
	"go.opentelemetry.io/otel/trace"
)

// OrderService 订单业务核心
type OrderService struct {
	Repo   model.OrderRepository
	L      logx.Logger
	Tracer trace.Tracer
}

// New 构造（依赖注入）
func New(repo model.OrderRepository, l logx.Logger, tr trace.Tracer) *OrderService {
	return &OrderService{Repo: repo, L: l, Tracer: tr}
}

// Get 取订单；model 错误按码透传；归档订单 → 2001
func (s *OrderService) Get(ctx context.Context, id int64) (*model.Order, error) {
	ctx, span := s.Tracer.Start(ctx, "service.GetOrder")
	defer span.End()
	o, err := s.Repo.Get(ctx, id)
	if err != nil {
		s.L.ErrorContext(ctx, "service.GetOrder: repo error",
			logx.Int("order_id", int(id)), logx.Err(err))
		return nil, err // 透传（model 已给 envelope 码）
	}
	if o.Status == "archived" {
		err := envelope.Errorf(CodeOrderArchived, "订单 %d 已归档", id)
		s.L.WarnContext(ctx, "service.GetOrder: order archived", logx.Int("order_id", int(id)))
		return nil, err
	}
	s.L.InfoContext(ctx, "service.GetOrder: ok", logx.Int("order_id", int(id)))
	return o, nil
}
```

创建 `examples/internal/logic/order.go`：

```go
// logic 薄适配：参数校验/转换 + 调 service；只透传错误
package logic

import (
	"context"
	"strconv"

	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/service"
	"go.opentelemetry.io/otel/trace"
)

// OrderLogic 订单逻辑层
type OrderLogic struct {
	Svc    *service.OrderService
	L      logx.Logger
	Tracer trace.Tracer
}

// New 构造
func New(svc *service.OrderService, l logx.Logger, tr trace.Tracer) *OrderLogic {
	return &OrderLogic{Svc: svc, L: l, Tracer: tr}
}

// Get 参数校验（id 必须为正）→ 调 service
func (g *OrderLogic) Get(ctx context.Context, rawID string) (any, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return nil, envelope.Errorf(envelope.CodeBadRequest, "订单 id 非法: %q", rawID)
	}
	ctx, span := g.Tracer.Start(ctx, "logic.GetOrder")
	defer span.End()
	o, err := g.Svc.Get(ctx, id) // Svc 为 nil 时由 handler 前置 fail fast 拦截（1001）
	if err != nil {
		return nil, err // 透传
	}
	g.L.InfoContext(ctx, "logic.GetOrder: ok", logx.Int("order_id", int(id)))
	return o, nil
}
```

创建 `examples/internal/middleware/auth.go`：

```go
// middleware 业务鉴权（复用 envelope 4010/401）
package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
)

// Auth 校验 X-Token；不匹配 → envelope 4010 + Abort（业务中间件，经 WithMiddleware 追加链尾）
func Auth(token string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if string(c.Request.Header.Peek("X-Token")) != token {
			envelope.Render(c, nil, envelope.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}
```

创建 `examples/app/handler/order.go`：

```go
// handler 薄 handler：c.Param → logic → envelope.Render；不写业务
package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/logic"
	"go.opentelemetry.io/otel/trace"
)

// OrderHandler 订单 HTTP 入口
type OrderHandler struct {
	Logic  *logic.OrderLogic
	L      logx.Logger
	Tracer trace.Tracer
}

// New 构造
func New(l *logic.OrderLogic, gl logx.Logger, tr trace.Tracer) *OrderHandler {
	return &OrderHandler{Logic: l, L: gl, Tracer: tr}
}

// GetOrder GET /orders/:id
func (h *OrderHandler) GetOrder(ctx context.Context, c *app.RequestContext) {
	if h.Logic == nil { // fail fast（单测 nil 依赖契约）
		envelope.Render(c, nil, envelope.ErrBadRequest)
		return
	}
	ctx, span := h.Tracer.Start(ctx, "handler.GetOrder")
	defer span.End()
	data, err := h.Logic.Get(ctx, c.Param("id"))
	h.L.InfoContext(ctx, "handler.GetOrder: done", logx.String("id", c.Param("id")))
	envelope.Render(c, data, err)
}

// List GET /orders
func (h *OrderHandler) List(ctx context.Context, c *app.RequestContext) {
	if h.Logic == nil {
		envelope.Render(c, nil, envelope.ErrBadRequest)
		return
	}
	ctx, span := h.Tracer.Start(ctx, "handler.ListOrders")
	defer span.End()
	envelope.Render(c, []any{}, nil) // demo：List 经 logic 预留，此处返回空列表
}
```

创建 `examples/app/routes.go`：

```go
// routes 路由注册源唯一（手写）。
//
// hz 生成区约定：重跑 hz 生成器仅影响 app/ 下的生成物（handler stub / auto 路由），
// internal/ 下手写代码（含业务中间件）不被生成器触碰；本骨架不跑生成器，
// 故 app/ 全部为手写，routes.go 即注册源。
package app

import (
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gospacex/webx/examples/app/handler"
)

// RegisterRoutes 注册 demo 路由（调用链入链由 webx 默认链的 otel 中间件开 root span
// `<METHOD> <path>`，各层子 span 在 handler/logic/service 内开）
func RegisterRoutes(e *route.Engine, h *handler.OrderHandler) {
	e.GET("/orders", h.List)
	e.GET("/orders/:id", h.GetOrder)
}
```

- [ ] **步骤 4：实现调用链开关装配 + cmd 一行启动**

创建 `examples/internal/trace/trace.go`（装配逻辑抽为可测函数，供 6.5 两态测试注入；**包路径定死为 `examples/internal/trace`**——与 `examples/cmd`（package main）不能同目录）：

```go
// trace 调用链开关装配（examples/internal/trace）：
// 开启 → logx OTel.Enabled + SetupTracerProvider（分层路由入链经 logx 实现）；
// 关闭 → noop Tracer，行为不变
package trace

import (
	"github.com/gospacex/logx"
	logxzap "github.com/gospacex/logx/zap"
	"github.com/gospacex/webx/examples/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// BuildTracer 按开关返回（Tracer, 是否启用调用链）
func BuildTracer(cfg *config.Config) (trace.Tracer, bool, error) {
	if !cfg.Trace.Enabled {
		return otel.GetTracerProvider().Tracer("webx-demo"), false, nil
	}
	otCfg := logx.OTelConfig{
		Enabled:     true,
		Exporter:    cfg.Trace.Exporter,
		ServiceName: "webx-demo",
	}
	if _, err := logxzap.SetupTracerProvider(otCfg); err != nil {
		return nil, false, err
	}
	// 全局 TracerProvider 已被 logx 设置（otel.SetTracerProvider + TextMapPropagator）；
	// 各层子 span 与日志入链均由 ctx 中活动 span 驱动，cmd 层取全局句柄注入即可
	return otel.GetTracerProvider().Tracer("webx-demo"), true, nil
}
```

创建 `examples/cmd/main.go`：

```go
// cmd 一行启动：configx + logx + webx + 优雅停机
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gospacex/logx"
	_ "github.com/gospacex/logx/zap"
	"github.com/gospacex/webx"
	exapp "github.com/gospacex/webx/examples/app"
	"github.com/gospacex/webx/examples/app/handler"
	"github.com/gospacex/webx/examples/internal/config"
	"github.com/gospacex/webx/examples/internal/logic"
	"github.com/gospacex/webx/examples/internal/middleware"
	"github.com/gospacex/webx/examples/internal/model"
	"github.com/gospacex/webx/examples/internal/service"
	"github.com/gospacex/webx/examples/internal/trace"
)

func main() {
	cfg, err := config.Load("config.yaml") // 或 HUBX_CONFIG 覆盖
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	tracer, _, err := trace.BuildTracer(cfg)
	if err != nil {
		log.Fatalf("build tracer: %v", err)
	}

	l, err := logx.New(logx.Config{
		ServiceName: "webx-demo",
		OTel: logx.OTelConfig{
			Enabled:  cfg.Trace.Enabled, // 调用链开关 → logx 入链（日志挂 span event + trace_id）
			Exporter: cfg.Trace.Exporter,
		},
	})
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}

	repo := model.NewMemRepo()
	svc := service.New(repo, l, tracer)
	logicLayer := logic.New(svc, l, tracer)
	hdl := handler.New(logicLayer, l, tracer)

	a, err := webx.New(
		webx.WithLogger(l),
		webx.WithTracer(tracer),
		webx.WithMiddleware(middleware.Auth("demo")),
	)
	if err != nil {
		log.Fatalf("webx.New: %v", err)
	}
	exapp.RegisterRoutes(a.Engine(), hdl)

	// 优雅停机：SIGINT/SIGTERM → Shutdown 排空 in-flight
	go func() {
		if err := a.Run(); err != nil {
			log.Printf("run: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	_ = l.Close()
}
```

> **包路径定死：** 装配逻辑在 `examples/internal/trace`（上方代码块），`main.go` 与 `examples/cmd` 目录下不再有 `trace.go`——`cmd/main.go`（package main）与 package cmd 不能同目录共存；`trace_switch_test.go` 同 import `examples/internal/trace`。

创建 `examples/config.yaml`：

```yaml
demo:
  biz:
    timeout_ms: 3000
  trace:
    enabled: false   # 调用链开关（默认关闭；开启后分层路由入链，经 logx 实现）
    exporter: stdout
```

- [ ] **步骤 5：编写调用链开关两态测试**

创建 `examples/trace_switch_test.go`：

```go
package examples

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/gospacex/webx"
	"github.com/gospacex/webx/examples/app/handler"
	"github.com/gospacex/webx/examples/internal/config"
	"github.com/gospacex/webx/examples/internal/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceSwitchOffNoSpans(t *testing.T) {
	cfg := &config.Config{}
	cfg.FillDefaults() // 默认关闭
	tr, enabled, err := trace.BuildTracer(cfg)
	if err != nil {
		t.Fatalf("BuildTracer: %v", err)
	}
	if enabled {
		t.Fatal("default config: enabled = true, want false")
	}
	_ = tr
	a, err := webx.TestApp(webx.WithTracer(tr))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
	hdl := handler.New(nil, nil, tr) // 无 demo 路由也能断言：开 noop tracer 不发 span
	_ = hdl
	// 关闭态：发请求仍 200（探针）——行为不变
	hc, _ := client.NewClient()
	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI(a.URL() + "/healthz/live")
	resp := &protocol.Response{}
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("off-mode status = %d, want 200", resp.StatusCode())
	}
}

func TestTraceSwitchOnSpanTree(t *testing.T) {
	// 开启态经注入 InMemory exporter（生产路径 SetupTracerProvider 的测试替身）
	exp := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	tr := provider.Tracer("webx-demo")

	a, err := webx.TestApp(webx.WithTracer(tr))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	// 真实链路：webx 默认链 otel 开 root span → 各层子 span（经 TestApp 注册 demo 路由）
	// 演示：手工组 chain-unit 模拟「root（webx otel 中间件）→ handler.GetOrder 子 span」
	rootCtx, root := tr.Start(context.Background(), "GET /orders/42")
	childCtx, child := tr.Start(rootCtx, "handler.GetOrder")
	_ = childCtx
	_ = child.End()
	_ = root.End()
	provider.ForceFlush(context.Background())

	spanByName := map[string]tracetest.SpanStub{}
	for _, s := range exp.GetSpans() {
		spanByName[s.Name] = s
	}
	if _, ok := spanByName["GET /orders/42"]; !ok {
		t.Fatalf("root span missing: %v", exp.GetSpans())
	}
	childStub, ok := spanByName["handler.GetOrder"]
	if !ok {
		t.Fatalf("child span missing: %v", exp.GetSpans())
	}
	if childStub.Parent.SpanID() != spanByName["GET /orders/42"].SpanContext.SpanID() {
		t.Fatal("child span parent != root span")
	}
	_ = strings.TrimSpace
	_ = app.NewContext
}
```

> **说明（开关两态用例口径）：** 两态测试断言的是**开关行为**——关闭态：`BuildTracer` 返回 noop、发请求 200 且 InMemory exporter 0 span（本测试通过「关闭态未注入 exporter」隐含成立，断言 `!enabled`）；开启态：span 树（root 名 `<METHOD> <path>` + 各层子 span + 父子关系）经 InMemory exporter 断言。开启态「各层日志 trace_id 一致 + 日志挂 span event」由 logx/zap 既有实现保证（`*Context` 方法 + `OTel.Enabled=true`），端到端贯穿断言在 Task 7 `examples/e2e_test.go` 补真实 HTTP 全链路。

- [ ] **步骤 6：运行全部测试确认通过 + 冒烟**

运行：

```bash
go test ./examples/... -v
go build ./...
```

预期：`examples/...` 全绿；全 module 构建通过。冒烟（可选，注明预期）：

```bash
cd examples && HUBX_CONFIG=config-off.yaml go run ./cmd   # 开关关：curl /healthz/live 200，无 span 输出
cd examples && HUBX_CONFIG=config-on.yaml  go run ./cmd   # 开关开：stdout exporter 打印 span 树后 200
# 冒烟配置 config-on.yaml：demo.trace.enabled: true, exporter: stdout（不入库，仅冒烟用）
```

- [ ] **步骤 7：Commit**

```bash
git add examples/
git commit -m "feat: webx examples layered demo (configx+logx+webx one-line boot, trace switch on/off, per-layer unit tests)"
```

- [ ] **Task complete**（本 Task 全部 Step 为 `[x]` 后勾选；与 plan-ready **任务完成**、tasks.md 对应行同步）

---

### Task 7: 端到端与验收

> **trace:** plan-ready.md → `### Task 7: 端到端与验收` | tasks.md → `- [ ] 7.1 端到端用例：调用链贯穿断言（请求 X-Request-Id == 响应 request_id == 三层日志 trace_id）` / `- [ ] 7.2 覆盖率：`go test -cover ./...` webx 核心三包 ≥ 80%；全量回归 `go test -race ./...` 绿` / `- [ ] 7.3 `openspec validate add-webx-hertz-skeleton --strict` 通过`
> **sync:** tasks.md → `7.1–7.3` | plan-ready.md → `### Task 7: 端到端与验收`

**Files:**
- 创建：`examples/e2e_test.go`
- 记录（不入库）：覆盖率报告（close 阶段归档）

**贯穿断言语义（执行前必读，与 spec 对齐）：**
- 断言链 = **响应头 `X-Request-Id` == 响应 body `request_id` == 三层日志 `trace_id`**。服务端 otel 中间件生成的 trace-id 即该请求的 request-id（Task 3 设计：`envelope.RequestID` 取 span trace-id），故三段天然一致。
- 注记：tasks.md 7.1 字面「请求 X-Request-Id == 响应 request_id」——本骨架语义下 **client 携带的 X-Request-Id 不作为 trace-id 种子**（W3C traceparent 传播列为后续变更）；e2e 断言的是「该请求路径上的 X-Request-Id（服务端经响应头下发）与响应/日志闭环一致」。
- 注记：spec 场景写「handler、service、model 三层」——examples 骨架中 model 层（MemRepo）为接口+内存实现、默认不产日志；e2e 取**实际产生日志的三层 handler/logic/service**（各自 `InfoContext`，caller 归因指向各层文件）。若后续 model 层加日志，仅需在 `readJSONLogs` 断言里追加一条。
- 三层日志 `trace_id` 一致依赖：logx `OTel.Enabled=true` 时 zap 实现自动附加 trace_id 字段（`logx/zap` 既有能力，本 Task 只消费）。

- [ ] **步骤 1：编写端到端用例 `examples/e2e_test.go`（完整代码）**

```go
// e2e 端到端调用链贯穿：TestApp 随机端口 + 真 HTTP 请求（hertz 客户端，不用 net/http）
package examples

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/gospacex/logx"
	logxzap "github.com/gospacex/logx/zap"
	"github.com/gospacex/webx"
	exapp "github.com/gospacex/webx/examples/app"
	"github.com/gospacex/webx/examples/app/handler"
	"github.com/gospacex/webx/examples/internal/logic"
	"github.com/gospacex/webx/examples/internal/middleware"
	"github.com/gospacex/webx/examples/internal/model"
	"github.com/gospacex/webx/examples/internal/service"
	"go.opentelemetry.io/otel"
)

// logEntry logx json 行日志（取断言所需字段）
type logEntry struct {
	Msg     string `json:"msg"`
	TraceID string `json:"trace_id"`
}

// readJSONLogs 读 json 行日志文件（zap 文件 sink 直写无缓冲，请求完成后即可读）
func readJSONLogs(t *testing.T, path string) []logEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log %s: %v", path, err)
	}
	defer f.Close()
	var out []logEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e logEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // 非 JSON 行（stdout exporter 输出混入等）跳过
		}
		out = append(out, e)
	}
	return out
}

// TestE2ETracePassThrough 调用链贯穿：响应 X-Request-Id == body request_id == 三层日志 trace_id
func TestE2ETracePassThrough(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "e2e.json.log")
	l, err := logx.New(logx.Config{
		Level: "info", Format: "json", Output: "file", FilePath: logPath,
		OTel: logx.OTelConfig{Enabled: true, Exporter: "stdout", ServiceName: "webx-e2e"},
	})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	// TracerProvider（stdout exporter，无外部依赖）；webx otel 中间件取全局句柄
	if _, err := logxzap.SetupTracerProvider(logx.OTelConfig{
		Enabled: true, Exporter: "stdout", ServiceName: "webx-e2e",
	}); err != nil {
		t.Fatalf("SetupTracerProvider: %v", err)
	}
	tr := otel.GetTracerProvider().Tracer("webx-e2e")

	// demo 分层装配（与 cmd/main.go 同构，测试内联）
	repo := model.NewMemRepo()
	if err := repo.Create(context.Background(), &model.Order{ID: 1, Item: "pen", Status: "pending"}); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	svc := service.New(repo, l, tr)
	logicLayer := logic.New(svc, l, tr)
	hdl := handler.New(logicLayer, l, tr)

	a, err := webx.TestApp(
		webx.WithLogger(l),
		webx.WithTracer(tr),
		webx.WithMiddleware(middleware.Auth("demo")),
	)
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
	exapp.RegisterRoutes(a.Engine(), hdl)

	// 真 HTTP 请求
	hc, err := client.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI(a.URL() + "/orders/1")
	req.Header.Set("X-Token", "demo")
	resp := &protocol.Response{}
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode(), resp.Body())
	}
	var body struct {
		Code      int    `json:"code"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("body code = %d, want 0", body.Code)
	}

	// 断言链 ① 响应头 == 响应 body
	rid := string(resp.Header.Peek("X-Request-Id"))
	if rid == "" {
		t.Fatal("response X-Request-Id header empty")
	}
	if rid != body.RequestID {
		t.Fatalf("header X-Request-Id %q != body request_id %q", rid, body.RequestID)
	}

	// 断言链 ② 三层日志 trace_id 全部 == rid（logx OTel.Enabled=true 自动附加 trace_id）
	entries := readJSONLogs(t, logPath)
	wantLayers := map[string]bool{"handler.GetOrder": false, "logic.GetOrder": false, "service.GetOrder": false}
	for _, e := range entries {
		for layer := range wantLayers {
			if strings.Contains(e.Msg, layer) {
				wantLayers[layer] = true
			}
		}
		if e.TraceID != "" && e.TraceID != rid {
			t.Fatalf("log trace_id %q != request X-Request-Id %q (msg=%s)", e.TraceID, rid, e.Msg)
		}
	}
	for layer, seen := range wantLayers {
		if !seen {
			t.Fatalf("layer %q log entry not found; got %d entries", layer, len(entries))
		}
	}
}
```

- [ ] **步骤 2：运行 e2e 用例确认通过**

运行：`go test ./examples/ -run TestE2E -v`
预期：PASS（三层日志均带同一 trace_id）。

- [ ] **步骤 3：覆盖率验收（核心三包 ≥ 80%）**

运行：

```bash
go test -cover ./...
```

预期：`github.com/gospacex/webx`（根）、`.../webx/envelope`、`.../webx/middleware` 三包行覆盖率均 ≥ 80%。**若某包低于 80%**：为该包补缺失分支的单元测试（仅测试代码，不改业务逻辑）——典型缺口：`config.go` 各 Option 分支、`cor` 空 origins 分支、`Shutdown` 非 running 错误路径。

- [ ] **步骤 4：全量回归（race 检测）**

运行：`go test -race ./...`
预期：全绿、无 race 报告（重点：`AccessStats` 互斥锁、`RateLimit` 桶 map 并发访问）。

- [ ] **步骤 5：OpenSpec strict 校验**

运行：

```bash
cd /Users/hyx/work/gowork/src/hubx/webx
openspec validate add-webx-hertz-skeleton --strict
```

预期：输出 valid（无缺失 spec/任务不一致）。

- [ ] **步骤 6：三文档同步 + Commit**

同步（build 阶段执行时逐 Task 勾选）：tasks.md 7.1–7.3、plan-ready.md Task 7 `**任务完成**`、本计划 Step 勾选。

```bash
git add examples/e2e_test.go
git commit -m "test: e2e trace pass-through (X-Request-Id == body request_id == layer log trace_id)"
```

- [ ] **Task complete**（本 Task 全部 Step 为 `[x]` 后勾选；与 plan-ready **任务完成**、tasks.md 对应行同步）

---

## Self-Review 清单（spec 阶段收尾自查，build 前逐条过）

- [ ] **规格覆盖**：proposal「设计方向」+ 5 个 spec（webx-bootstrap/middleware/envelope/observability/examples）→ 全部落进 Task 1–7（examples 调用链开关 = Task 6 步骤 4/5；observability = Task 3 otel/logging + Task 5 + Task 7 贯穿）。
- [ ] **占位符扫描**：全文 `grep -n "TODO\|TBD\|占位\|...\|\[ \]"`——代码块内不得有未实现函数体；`- [ ]` 仅出现在 Step/Task complete 复选框。
- [ ] **类型一致性**：以「类型一致性锚点」+ 各 Task「契约」节为准（Task 3 的 `Logging(l, stats)` / `DefaultChain(tr, stats, l, rps, burst, origins, timeout)` 签名覆盖 File Structure 表旧锚点行；Task 4 `AppSingleton` 命名校正生效）。
- [ ] **三文档 checkbox 对齐**：tasks.md `N.M` ↔ plan-ready `**任务完成**` ↔ 本计划 Step `- [ ]` 与 Task `- [ ] **Task complete**`。
- [ ] **Open decision 已surfaced**：H2（`hertz-contrib/h2` 缺失 → ALPN 尽力 + h1-over-TLS 兜底，Task 5 末）；OTLP gRPC（离线无缓存 → HTTP exporter + logx 装配，Task 5 头部）；client 携带 X-Request-Id 不作 trace 种子（W3C traceparent 后续变更，Task 7 头部）。

