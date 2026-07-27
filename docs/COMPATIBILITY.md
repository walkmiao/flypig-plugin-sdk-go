# 兼容性与升级

## 当前基线

```text
SDK: 1.3.3
Plugin API: 1.1.0
Go: 1.25.0
```

## 兼容原则

- SDK patch 版本不应破坏公共 API。
- SDK minor 版本可以增加向后兼容 API。
- Plugin API 主版本决定 RPC 合同兼容边界。
- 插件不要求和 Data Plane 使用相同 SDK 版本，只要求握手合同兼容。

## 升级

```bash
go get github.com/walkmiao/flypig-plugin-sdk-go@v1.3.3
go mod tidy
go test ./...
go test -race ./...
```

然后使用匹配的 Developer Kit 重新执行黑盒一致性测试和 `.fpp` 构建。

## 本地 replace

本地开发可以使用 replace，但发布前应删除：

```bash
go mod edit -dropreplace=github.com/walkmiao/flypig-plugin-sdk-go
go mod tidy
```
