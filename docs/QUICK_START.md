# SDK 快速开始

## 1. 添加依赖

```bash
go get github.com/walkmiao/flypig-plugin-sdk-go@v1.3.3
go mod tidy
```

要求 Go `1.25.0`。

## 2. 创建插件

```go
package main

import (
    pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
    flyruntime "github.com/walkmiao/flypig-plugin-sdk-go/runtime"
)

type Plugin struct {
    *flyruntime.BasePlugin
}

func NewPlugin() *Plugin {
    return &Plugin{BasePlugin: flyruntime.NewBasePlugin(flyruntime.Metadata{
        Code:                "my-collector",
        Name:                "My Collector",
        Vendor:              "My Company",
        Description:         "Example collector",
        PluginVersion:       "0.1.0",
        ConfigSchemaVersion: 1,
        PointSchemaVersion:  1,
        EventSchemaVersion:  1,
        Language:            "go",
        Capabilities: []pluginapi.Capability{
            pluginapi.CAPABILITY_HEALTH,
            pluginapi.CAPABILITY_STATUS,
            pluginapi.CAPABILITY_TASK_MANAGEMENT,
            pluginapi.CAPABILITY_TELEMETRY,
        },
        ConnectionSemantics: []pluginapi.ConnectionSemantics{
            pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE,
        },
    })}
}
```

BasePlugin 只提供安全默认值。你仍需覆盖任务管理和采集方法。

## 3. 添加一致性测试

```go
func TestPluginAPIConformance(t *testing.T) {
    testkit.RunConformance(t, NewPlugin(), "my-collector")
}
```

## 4. 运行测试

```bash
go test ./...
go test -race ./...
```

## 5. 使用完整 Developer Kit 构建

独立 SDK 不包含 `.fpp` 构建工具。标准构建由 Plugin Developer Kit 提供。
