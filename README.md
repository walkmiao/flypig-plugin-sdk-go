# FlyPig Plugin SDK for Go

`github.com/walkmiao/flypig-plugin-sdk-go` is the public Go SDK for FlyPig Plugin API v1.
It contains generated API bindings, the standard HashiCorp go-plugin runtime boundary,
conformance helpers, and a runnable demo collector.

## Packages

- `pluginapi`: generated Plugin API v1 protobuf and gRPC bindings.
- `runtime`: server/client helpers, handshake, metadata, results, and SecretRef helpers.
- `testkit`: reusable conformance assertions for plugin implementations.
- `cmd/plugin-conformance-check`: black-box executable conformance check.
- `examples/demo-collector`: minimal buildable collector.

## Existing plugin project

Keep the SDK outside the plugin project, then point the plugin module at it:

```go
require github.com/walkmiao/flypig-plugin-sdk-go v0.0.0
replace github.com/walkmiao/flypig-plugin-sdk-go => ../flypig-plugin-sdk-go
```

Run:

```bash
go test ./...
```

The SDK does not expose FlyPig control-plane, data-plane, IEC104 adapter, or legacy
plugin contract packages.
