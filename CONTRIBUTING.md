# Contributing

## 公共 API 原则

- `pluginapi` 由 protobuf 合同生成，不手工编辑。
- `runtime` 和 `testkit` 是公开 API，变化必须评估兼容性。
- 不得引入 Control Plane、Data Plane 或内置协议插件内部依赖。
- 新能力必须先具备 Plugin API、宿主和测试闭环。

## 验证

```bash
go test ./...
go test -race ./...
```

在 FlyPig 规范源仓库还需运行：

```bash
python3 hack/scripts/verify_plugin_go_sdk.py
python3 hack/scripts/verify_plugin_docs.py
```

## 生成代码

protobuf 生成文件只能通过标准生成脚本更新，并同时审查合同锁变化。
