# Stoat 架构说明

## 目标

Stoat 是本地、可解释的 macOS Persistence Inspector。扫描默认只读；launchd 状态修改通过独立 action 层完成。可靠性优先于扫描数量；无法确认的数据标记为未知，不推断为安全或恶意。

## 数据流

```text
macOS sources
  -> collectors
  -> source parsers
  -> PersistenceItem
  -> deduplication
  -> runtime / attribution / file-signature enrichments
  -> risk rules
  -> TUI / table / JSON
```

### Collectors

Collector 只负责定位来源和取得原始数据。每个 Collector 独立返回 `Items + Warnings`，不以单点失败终止扫描。

- `LoginItems`：执行 `sfltool dumpbtm`
- `Launchd`：枚举受控目录，执行 `plutil -convert json`
- `Cron`：读取当前用户 crontab 和 `/etc/crontab`

### Parsers

Parser 不访问系统，输入固定字节并产生领域对象，便于 fixture 和 table-driven test。未知 plist/BTM 字段被忽略，已知字段类型错误会产生局部 warning。

### Unified model

所有来源统一为 `PersistenceItem`。分类是多值：同一 launchd job 可同时属于 Startup、Scheduled 和 Background。TUI、JSON、过滤器和规则引擎不依赖来源格式。

### Enrichment

Enricher 按固定顺序执行，每个阶段使用有界并发且局部失败只生成 warning：

1. Runtime：`launchctl print-disabled` 每个 domain 一次，逐项读取加载、运行、PID 和退出状态。
2. Attribution：只接受来源 Bundle ID、`.app` 路径和 `Info.plist` 作为证据，不根据 label 猜测。
3. Signature：只处理绝对路径；先读取文件属性，再在 macOS 上执行 `codesign`。

并发上限为 4，避免大量进程争用。未执行检查和检查后得到否定结果是不同状态。

### Risk engine

规则是纯函数：`PersistenceItem -> []Finding`。每条 finding 包含稳定规则 ID、分数、原因和证据，最终分数限制在 0–100。风险例外只能按 `item_id + rule_id` 精确匹配正向 finding；被屏蔽项仍保留在输出中用于审计。

```text
0–19   Trusted
20–39  Normal
40–69  Attention
70–100 High
```

默认基线为 20，Apple 签名与系统路径降低风险；临时目录、其他用户可写、root unsigned daemon、下载后管道执行等提高风险。等级表示人工复核优先级，不代表恶意软件结论。

## 信任边界

1. 配置文件和系统命令输出均是不可信输入。
2. 不通过 Shell 解释任何扫描结果。
3. 可执行命令及路径在代码中固定，不读取环境变量覆盖。
4. 配置文件必须是普通文件且受大小限制。
5. 所有外部命令和完整扫描均有 deadline。
6. Collector、Parser、Enricher 和 Risk 层没有写入系统状态的能力。
7. 唯一写入边界是 action 层，必须经过 plan、备份、确认、执行、验证和审计。

## 演进约束

- 运行状态：作为独立 enrichment 加入，不能侵入 parser。
- App Attribution：以 Bundle ID、Team ID、可执行路径建立证据链，不按 label 猜测。
- 新持久化来源：新增 Collector + Parser，不能在 UI 写来源特例。
- 禁用/恢复：位于独立 action 层；先摘要校验和备份，再操作、验证，失败自动回滚。
- 原生 GUI：复用 app/model/risk，不改扫描协议；JSON schema 需要版本号后再承诺稳定。

## 阶段计划

### v0.1（已完成）

基础扫描、解析、风险引擎、TUI/JSON、安全命令执行、单元测试和 CI。

### v0.2（已完成）

launchctl runtime、App Attribution、JSON/CSV 原子导出、macOS 13–15 fixture、性能基准。

### v0.3（已完成）

扫描快照与差异对比、严格规则例外 schema、Homebrew HEAD 安装、双架构发布、校验和与 Sigstore 无密钥签名。

Apple Developer ID 签名和公证属于后续发布基础设施工作，需要项目所有者证书，不在仓库中保存凭据。

### v0.5（已完成）

launchd 停用、隔离、恢复、确认令牌、备份、失败回滚、恢复验证和操作审计。BTM 与 cron 保持只读。

### v0.8（已完成）

保守轮询监控、持久化变更事件、历史查询、Unified Log 诊断和 Homebrew service。任何带 warning 的扫描均不推进监控基线。

### v1.0（已完成）

固化命令和 JSON schema v1，增加经过校验的一键安装器、GitHub / `lighting.pub` / 国内镜像分发约定、离线安装集成测试及完整发布待办。公开 Homebrew、域名部署和 Apple 公证是外部发布 Feature，不阻塞源码达到 v1 功能边界。

### v1.1（已完成）

TUI 增加 launchd 操作菜单；CLI 与 TUI 统一支持停用、启用、隔离、可恢复删除和受限应用卸载。卸载仅移动证据链明确的 App Bundle 到废纸篓，操作仍经过确认、摘要复核、备份、审计、验证和失败回滚。
