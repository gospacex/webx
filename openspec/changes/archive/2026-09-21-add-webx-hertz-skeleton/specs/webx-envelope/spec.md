# webx-envelope Specification

## ADDED Requirements

### Requirement: 统一响应封装
系统 SHALL 提供 `envelope.Response{Code, Message, Data, RequestID}` 与渲染函数 `envelope.Render(c *app.RequestContext, data any, err error)`：`err` 为 nil 时 code=0、data 填充；`err` 非 nil 时 data 为 nil、code/message 取自错误。响应内容类型 SHALL 为 `application/json; charset=utf-8`。

#### Scenario: 成功响应
- **WHEN** `Render(c, order, nil)` 且请求携带 request_id
- **THEN** 响应 body 为 `{"code":0,"message":"ok","data":...,"request_id":"..."}`，HTTP 200

#### Scenario: 失败响应
- **WHEN** `Render(c, nil, envelope.Errorf(envelope.CodeNotFound, "用户 %d 不存在", 42))`
- **THEN** 响应 body code=1004、message 含格式化文案、data 为 null，HTTP 状态按错误码映射

### Requirement: 错误码体系
`envelope` SHALL 定义分段错误码常量与 `*envelope.Error` 类型（实现 `error` 接口，携带 code 与 message），并支持 `errors.Is` 判定：
- `1xxx` 通用/数据类：`ErrBadRequest=1001`、`ErrNotFound=1004`、`ErrConflict=1009`
- `2xxx` 业务类：`ErrBizRule=2000`（业务方扩展 2xxx 段）
- `4xxx` 网关语义：`ErrUnauthorized=4010`、`ErrRateLimited=4290`
- `5xxx` 服务端：`ErrInternal=5000`、`ErrPanic=5001`、`ErrTimeout=5002`

#### Scenario: errors.Is 判定
- **WHEN** `errors.Is(err, envelope.ErrNotFound)` 且 err 为 code=1004 的 `*envelope.Error`
- **THEN** 返回 true；code 不同的错误返回 false

#### Scenario: 错误码表完整
- **WHEN** 遍历 `envelope.CodeNames()` 导出的全部预置码
- **THEN** 每个码有稳定文案，且 HTTP 映射表（`CodeToStatus`）覆盖全部预置码

### Requirement: 错误透传约定
业务各层 SHALL 只透传 `*envelope.Error`，中间层（logic）SHALL 不得重新包装或吞掉错误；仅 service 层可把 model 层错误转换为业务码。

#### Scenario: model 错误上抛
- **WHEN** model 仓储返回 `ErrNotFound` 且 service 未转换
- **THEN** handler 渲染响应 code=1004，透传路径中 request_id 保持不变
