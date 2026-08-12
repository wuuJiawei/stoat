# Changelog

## v1.0.0

- 增加 GitHub Release、`stoat.lighting.pub` 和可配置 GitHub 代理的一键安装方案。
- 安装器自动识别 Apple Silicon / Intel，强制 HTTPS、SHA-256 校验、归档路径白名单和原子替换，默认不使用 `sudo`。
- Release 同步发布安装脚本和 `latest.txt`，CI 增加 ShellCheck 与离线安装集成测试。
- 固化 v1 功能边界、安装文档、发布待办和 Homebrew / 域名 / Apple 公证的外部依赖。

## v0.8.0

- 增加保守轮询监控；扫描出现 warning 时不更新基线，避免把采集失败误报为删除。
- 增加最多 1000 条私有变更事件、`watch` JSON 流和 `changes` 历史查询。
- 增加 launchd 运行状态、退出码、风险结果与 Unified Log 联合诊断。
- Homebrew Formula 增加 service 定义，并补充 Private Tap、公开 Tap 与 homebrew/core 发布边界。

## v0.5.0

- 增加 launchd 停用、隔离、恢复和操作审计；BTM 与 cron 保持只读。
- 增加配置 SHA-256 绑定的双阶段确认，配置变化后拒绝执行旧计划。
- 增加私有备份、路径与符号链接保护、失败回滚和恢复后验证。
- 系统项目要求调用者已是 root，工具自身不调用 `sudo`。

## v0.3.0

- 增加严格校验的扫描快照和新增、删除、配置变更 diff。
- 风险规则增加稳定 ID、分数与证据，TUI / JSON 展示同一结果。
- 增加按 item ID + rule ID 精确匹配、可过期且保留审计记录的风险例外策略。
- 增加 Homebrew HEAD Formula、双架构 Release、校验和与 Sigstore 无密钥签名。

## v0.2.0

- 增加 launchctl 加载、运行、PID、退出码和禁用状态检查。
- 增加基于 App Bundle 路径和 Info.plist 的可解释应用归属。
- 增加 JSON / CSV 原子导出及 CSV 公式注入防护。
- 增加 macOS 13 / 14 / 15 BTM fixture 和解析 benchmark。

## v0.1.0

- 建立只读扫描、统一模型、风险规则、TUI / JSON、安全命令执行和 CI 基线。
