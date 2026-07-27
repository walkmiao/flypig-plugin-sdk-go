# FlyPig Go SDK 文档

独立 Go SDK 文档面向已经拥有插件项目、希望直接使用 SDK API 的开发者。

## 阅读顺序

1. [SDK 快速开始](QUICK_START.md)
2. [Runtime API 指南](RUNTIME_API_GUIDE.md)
3. [TestKit 指南](TESTKIT_GUIDE.md)
4. [SecretStore 指南](SECRETSTORE_GUIDE.md)
5. [兼容性与升级](COMPATIBILITY.md)

可运行参考：

```text
examples/demo-collector/
```

## 公共包

```text
pluginapi
runtime
testkit
cmd/plugin-conformance-check
```

SDK 不公开 FlyPig Control Plane、Data Plane 或内置协议插件的内部实现。
