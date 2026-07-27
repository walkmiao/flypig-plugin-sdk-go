# SecretStore 指南

## 创建

```go
store, err := runtime.NewSecretStore(refs, bundle, time.Now().UTC())
if err != nil {
    return err
}
defer store.Close()
```

校验内容：

- SecretReference 的 name/ref 必填。
- alias 不重复。
- SecretValue 必须匹配已声明引用。
- value alias 不重复。
- 未过期。
- required Secret 已交付。

## 获取

```go
value, ok := store.Bytes("credential")
expiresAt, hasExpiry := store.ExpiresAt("credential")
```

`Bytes` 返回副本。调用者负责其后续生命周期。

## 清理

```go
store.Close()
```

Close 会覆盖并删除 Store 自己持有的字节。调用者复制出的值也必须主动清理。

## 禁止行为

- 日志记录值。
- 持久化值。
- 放入 telemetry、extensions、errors。
- 忽略 `expires_at`。
- 任务停止后继续保留副本。
