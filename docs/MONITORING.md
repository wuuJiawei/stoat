# 变更监控与诊断

## 监控

```bash
stoat watch --interval 30s
stoat changes --limit 50
```

第一次扫描建立基线。后续只记录持久化配置、签名、归属、文件属性和禁用状态变化；PID 和运行/停止切换不会产生事件。

任一采集或 enrichment 出现 warning 时，当前扫描视为不完整：不更新基线，也不产生“删除”事件。事件以 `0600` 独立 JSON 文件保存，默认最多保留 1000 条；更旧事件自动清理。

脚本集成使用 NDJSON：

```bash
stoat watch --json
stoat changes --json
```

通过 Homebrew 安装后可持续运行：

```bash
brew services start stoat
brew services stop stoat
```

## 诊断

```bash
stoat diagnose <id-or-label> --last 1h --limit 100
stoat diagnose <id-or-label> --last 1h --limit 100 --json
```

诊断结果合并当前 launchctl 状态、最近退出码、风险分数和按可执行文件进程名过滤的 Unified Log。日志窗口限制为 1 分钟至 24 小时，条目限制为 1–500，原始命令输出限制为 8 MiB。

Unified Log 可能包含应用私有数据；公开提交报告前必须脱敏。
