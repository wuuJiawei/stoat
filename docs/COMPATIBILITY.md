# macOS 兼容性策略

Stoat 最低支持 macOS 13。涉及 `sfltool` 和 `launchctl` 的诊断输出不承诺字段稳定，因此采用以下约束：

- 解析器忽略未知字段，只读取具名且可验证的字段。
- macOS 13、14、15 的 BTM 脱敏样本进入 fixture 回归测试。
- 新系统版本先补充脱敏 fixture，再调整解析器。
- 无法确认的运行状态和应用归属保持 Unknown，不推断为安全或恶意。
- macOS 14 / 15 由 GitHub Actions runner 执行真实只读扫描；macOS 13 通过 fixture 与维护者设备回归。CI fixture 不替代真机验证。

提交真实输出前必须删除用户名、主目录、设备标识、第三方私有路径和完整任务参数。
