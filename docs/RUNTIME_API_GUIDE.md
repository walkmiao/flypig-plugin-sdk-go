# Runtime API 指南

## BasePlugin

`runtime.BasePlugin` 提供：

- Plugin API v1.1 握手。
- Metadata 和 Info。
- 生命周期状态存储。
- runtime epoch。
- 基础 Health/Status。
- 未实现可选能力的标准错误结果。

嵌入后覆盖协议相关方法。

## Metadata

`Metadata` 中以下字段必须与 Manifest 一致：

```text
Code
Name
Vendor
Description
PluginVersion
ConfigSchemaVersion
PointSchemaVersion
EventSchemaVersion
Capabilities
ConnectionSemantics
```

## 结果辅助

```go
runtime.Success()
runtime.Failure(code, message, retryable)
runtime.Unsupported("capability")
```

预期业务错误使用 `OperationResult`。不要用 panic 或错误字符串代替标准错误码。

## Server

使用 SDK 提供的标准 server 启动 HashiCorp go-plugin 和 gRPC 服务。不要自行改变 handshake magic、协议版本和插件 map key。

查看完整入口：

```text
examples/demo-collector/main.go
```

## Client

`runtime.Client` 面向宿主或测试工具，不建议业务插件自行连接另一个插件进程。

## 生命周期

BasePlugin 的默认实现适合示例和最小测试，不适合直接用于生产协议：

- `ValidateConfig` 默认成功。
- `ApplyTasks` 默认不启动任务。
- `Collect` 默认返回不支持。
- `StreamEvents` 默认等待取消。

生产插件必须实现声明能力。
