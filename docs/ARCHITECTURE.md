# Stoat 架构说明

## 目标

Stoat V1 是本地、只读、可解释的 macOS Persistence Inspector。可靠性优先于扫描数量；无法确认的数据标记为未知，不推断为安全或恶意。

## 数据流

```text
macOS sources
  -> collectors
  -> source parsers
  -> PersistenceItem
  -> deduplication
  -> file/signature enrichment
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

签名模块只处理绝对路径；先读取文件属性，再在 macOS 上执行 `codesign`。并发上限为 4，避免大量进程争用。未执行签名检查与检查后确认 unsigned 是两个不同状态。

### Risk engine

规则是纯函数：`PersistenceItem -> []Finding`。每条 finding 包含分数与原因，最终分数限制在 0–100。

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
6. V1 没有写入系统状态的能力。

## 演进约束

- 运行状态：作为独立 enrichment 加入，不能侵入 parser。
- App Attribution：以 Bundle ID、Team ID、可执行路径建立证据链，不按 label 猜测。
- 新持久化来源：新增 Collector + Parser，不能在 UI 写来源特例。
- 禁用/恢复：只能作为独立 action 层；先快照、校验、操作、验证，绝不直接删除 plist。
- 原生 GUI：复用 app/model/risk，不改扫描协议；JSON schema 需要版本号后再承诺稳定。

## 阶段计划

### v0.1（本次）

基础扫描、解析、风险引擎、TUI/JSON、安全命令执行、单元测试和 CI。

### v0.2

launchctl runtime、App Attribution、导出、跨 macOS 13–最新版本 fixture、性能基准。

### v0.3

扫描快照与差异对比、规则配置 schema、Homebrew 安装和签名发布。

禁用/恢复不进入前三个里程碑，需单独安全评审。
