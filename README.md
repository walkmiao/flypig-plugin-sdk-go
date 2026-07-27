# FlyPig Plugin SDK for Go

`github.com/walkmiao/flypig-plugin-sdk-go` 是 FlyPig Plugin API v1 的公开 Go SDK，提供生成的 API 绑定、HashiCorp go-plugin + gRPC 运行边界、安全默认实现、一致性测试工具和可运行示例。

## 当前基线

```text
SDK:        1.3.3
Plugin API: 1.1.0
Go:         1.25.0
```

## 公共包

| 包 | 用途 |
|---|---|
| `pluginapi` | Plugin API v1 protobuf 和 gRPC 绑定 |
| `runtime` | BasePlugin、服务端/客户端、结果和 Secret 工具 |
| `testkit` | 内存 gRPC 一致性测试 |
| `cmd/plugin-conformance-check` | 真实插件进程黑盒检查 |
| `examples/demo-collector` | 最小可运行采集器 |

## 安装

```bash
go get github.com/walkmiao/flypig-plugin-sdk-go@v1.3.3
go mod tidy
```

本地联调可以临时使用：

```go
replace github.com/walkmiao/flypig-plugin-sdk-go => ../flypig-plugin-sdk-go
```

正式发布前删除本地 `replace`。

## 验证

```bash
go test ./...
go test -race ./...
```

## 文档

从 [SDK 文档首页](docs/README.md) 开始：

- [快速开始](docs/QUICK_START.md)
- [Runtime API 指南](docs/RUNTIME_API_GUIDE.md)
- [TestKit 指南](docs/TESTKIT_GUIDE.md)
- [SecretStore 指南](docs/SECRETSTORE_GUIDE.md)
- [兼容性与升级](docs/COMPATIBILITY.md)

完整 `.fpp` 构建、Manifest 生成和上传分发流程由 Plugin Developer Kit 提供。

## 边界

SDK 不公开 FlyPig Control Plane、Data Plane、内置 IEC104 适配器、数据库模型或 legacy 插件合同。业务插件不得绕过 Plugin API 依赖平台内部代码。

## 仓库说明

- [版本记录](CHANGELOG.md)
- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
