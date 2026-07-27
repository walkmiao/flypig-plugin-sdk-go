# TestKit 指南

## RunConformance

```go
testkit.RunConformance(t, plugin, "expected-code")
```

测试使用内存 gRPC server，验证：

- API 版本协商。
- session/requestResponse/listener 宿主语义支持。
- Info 身份和版本。
- ValidateConfig。
- Init、Start、Health、Status、Stop、Shutdown。

## TestKit 不验证的内容

- 真实网络连接。
- task revision 行为。
- 遥测值正确性。
- 事件序列和 Ack。
- 命令执行。
- 设备和点位发现。
- Secret 清理。
- 交互日志脱敏。

这些必须由插件自己的测试覆盖。

## 黑盒检查

SDK 包含：

```text
cmd/plugin-conformance-check
```

Developer Kit 构建工具会用它启动真实可执行文件进行检查。可以手工运行：

```bash
go run ./cmd/plugin-conformance-check \
  --plugin /path/to/plugin-binary \
  --expected-code my-collector \
  --expected-version 0.1.0 \
  --config /path/to/conformance-config.json \
  --config-schema-version 1
```
