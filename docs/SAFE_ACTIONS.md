# Stoat 安全操作协议

## 支持范围

`disable`、`quarantine` 和 `restore` 只支持以下配置：

- `~/Library/LaunchAgents/*.plist`
- `/Library/LaunchAgents/*.plist`
- `/Library/LaunchDaemons/*.plist`

`/System/Library`、Background Task Management 和 cron 永久不进入当前写入边界。

## 两阶段确认

第一次执行只输出计划，不修改系统：

```bash
stoat disable <id-or-label>
```

计划包含配置 SHA-256 和确认令牌。再次扫描后，只有路径、label、domain 和配置内容均未变化时令牌才有效：

```bash
stoat disable <id-or-label> --confirm <token>
```

## 操作语义

- `disable`：先备份配置，再执行 `launchctl bootout` 和 `launchctl disable`，原 plist 保留。
- `quarantine`：完成停用后，将 plist 以不覆盖方式移至同目录的随机隔离路径。
- `restore`：校验备份或隔离文件摘要，恢复配置，执行 `launchctl enable`；原先已加载的项目再执行 `bootstrap`。

每一步均记录到 `~/Library/Application Support/Stoat/operations/<id>/manifest.json`。目录权限为 `0700`，文件为 `0600`。

## 失败与恢复

执行失败时 Stoat 尝试恢复原文件、启用状态和原加载状态。若回滚不完整，操作状态记为 `failed`，不得继续自动操作；使用以下命令取得审计记录后人工处理：

```bash
stoat audit <operation-id>
```

Stoat 不调用 `sudo`。系统级项目必须由用户明确以 root 身份启动命令。
