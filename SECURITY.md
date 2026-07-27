# Security Policy

SDK 安全问题包括 SecretStore、握手协商、进程服务边界、错误信息泄漏和测试工具绕过。

插件开发者应遵守：

- Secret 只保存在必要的进程内存生命周期。
- 不将 Secret 写入日志、错误、遥测和扩展。
- 使用标准 server helper，不修改 handshake 参数。
- 对网络、文件、事件批次和输入大小设置边界。

安全报告中不要包含真实生产凭证。
