# Contributing

## Required checks

```bash
make verify
```

提交应保持单一职责。Parser 和风险规则必须提供 table-driven test 或 fixture；调用 macOS 命令必须复用 `internal/executil`，禁止新增 `sh -c`、动态命令路径、无超时进程或隐式提权。

## Code rules

- Collector 负责采集，Parser 负责解释，Risk Rule 负责评分，TUI 只负责展示。
- 错误需保留来源和路径上下文；允许部分结果时返回 warning。
- 说明“为什么”的注释优先，不复述代码。
- 领域模型变化需同步 JSON 输出、fixture 和架构文档。
- V1 禁止系统写入、删除、禁用和 `sudo`。
