# Stoat 项目章程

## 定位

Stoat 是面向普通开发者和高级用户的 macOS 自动运行项检查器。它汇总登录项、launchd 和 cron，提供可解释的复核优先级。

## V1 成功标准

- 在 macOS 13+ 无管理员权限完成只读扫描。
- 单个文件或系统命令失败时继续扫描，并明确展示 warning。
- launchd、cron 和 BTM 固定样本可重复解析。
- JSON 输出与 TUI 使用同一领域模型和风险结果。
- 不存在 Shell 注入、PATH 劫持、无限输出、无限等待和系统写入路径。
- CI 执行 format、vet、race test 及 Darwin arm64/amd64 交叉构建。

## 非目标

- 判断病毒或替代专业恶意软件检测。
- 自动禁用、删除或修复任务。
- 逆向修改 Background Task Management 数据库。
- V1 请求 root、Full Disk Access 或 Endpoint Security entitlement。

## 决策原则

1. 证据不足时显示 Unknown，不猜测。
2. 领域逻辑写在 Go，系统命令只采集信息。
3. 新来源通过 Collector + Parser 接入，不在 UI 添加来源特例。
4. 风险规则必须有测试、分数和用户可见原因。
5. 任何系统状态写入能力必须单独立项和安全评审。
