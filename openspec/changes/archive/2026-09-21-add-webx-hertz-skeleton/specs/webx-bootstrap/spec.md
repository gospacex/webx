# webx-bootstrap Specification

## ADDED Requirements

### Requirement: 显式构造与选项
系统 SHALL 提供 `webx.New(opts ...Option)` 返回 `*webx.App`；`App` 内嵌 `*hertz.App` 并暴露 `Run`/`Shutdown`。选项 SHALL 包括 `webx.Config(cfg)`、`webx.WithLogger(logx.Logger)`、`webx.WithTLS(certFile, keyFile)`、`webx.WithMiddleware(...)`；未提供配置时使用内置默认值（随机端口 0、自动挂载 health 端点）。全程 SHALL 不依赖 `net/http`。

#### Scenario: 最小启动
- **WHEN** 调用 `webx.New(webx.Config(cfg))` 后执行 `app.Run()`
- **THEN** 服务在 cfg 指定端口（Hertz 地址）监听，health 端点可用，进程内无 `net/http` 栈

#### Scenario: 缺省配置
- **WHEN** 调用 `webx.New()` 不传任何选项
- **THEN** 以默认配置（随机端口 0）创建 App，不 panic

### Requirement: 单例便捷入口
系统 SHALL 提供 `webx.App()`：首次调用时以默认配置 `New` 并缓存，之后返回同一实例。重复调用 SHALL 返回同一指针。

#### Scenario: 重复调用返回同一实例
- **WHEN** 连续两次调用 `webx.App()`
- **THEN** 返回的指针相等（同一 `*App`）

#### Scenario: 测试隔离
- **WHEN** 需要多实例时调用 `webx.New(...)` 或 `webx.TestApp()`
- **THEN** 不影响 `webx.App()` 缓存实例

### Requirement: 测试逃生舱
系统 SHALL 提供 `webx.TestApp(opts ...Option)`：创建独立实例（不入单例缓存），监听随机端口（地址 127.0.0.1:0），提供 `URL()` 获取实际地址，测试结束可 `Shutdown` 释放端口。

#### Scenario: 随机端口测试
- **WHEN** `app := webx.TestApp()` 后取 `app.URL()`
- **THEN** 返回形如 `http://127.0.0.1:<非 0 端口>` 的地址，且 Shutdown 后端口释放

### Requirement: 优雅停机
`App.Shutdown(ctx)` SHALL 停止接受新连接并等待 in-flight 请求处理完成（受 ctx 超时约束）；`cmd` 层监听 SIGINT/SIGTERM 后调用 Shutdown。

#### Scenario: in-flight 排空
- **WHEN** 有请求处理中时触发 Shutdown（ctx 5s）
- **THEN** 已开始的请求正常返回响应，无连接被直接切断；超时后剩余请求被取消
