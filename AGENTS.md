# Stoat Agent Guide

本文件是 AI Agent 与自动化工具参与 Stoat 开发时的共同约束。仓库文档、代码与测试事实优先于推测；若规则与当前实现冲突，先核实并在 PR 中说明。

## 产品定位

Stoat 是终端优先的 macOS 持久化检查与管理工具，核心任务是回答：

1. 哪些项目会在登录、开机、定时或后台自动运行？
2. 它们从哪里来、当前状态如何、为什么值得复核？
3. 在可验证、可审计、可恢复的前提下，用户能做什么？

### 应该做

- 统一采集 Login Items、BTM、launchd 与 cron，并保留来源证据。
- 提供简洁的分类导航、详情解释、机器可读输出和稳定 Schema。
- 让 launchd 操作遵循计划、确认、备份、执行、验证、审计和回滚闭环。
- 保持风险规则可解释：稳定规则 ID、明确分数、具体证据和可过期例外。
- 对不稳定的 macOS 输出保持兼容，未知内容跳过或报告 warning。

### 不应该做

- 不把 Stoat 扩展成杀毒软件、通用清理器、包管理器或系统优化工具。
- 不写入 BTM 私有数据库，不整体重写 crontab，不修改 Apple 系统任务。
- 不根据模糊名称、厂商前缀或通配符猜测应用归属及关联文件。
- 不删除应用缓存、配置、账号、凭据、文档或其他用户数据。
- 不自行提权，不调用 `sudo`，不引入隐藏的后台修改或未经确认的破坏性操作。

产品归属不明确时，优先缩小范围、保持只读或明确标记为不支持。

## 仓库结构

- `cmd/stoat/`：CLI 入口、参数解析和命令编排；不要放领域实现。
- `internal/collector/`：调用受控 macOS 接口并采集原始数据。
- `internal/parser/`：将 plist、cron、BTM 等输入转换为领域模型。
- `internal/app/`：扫描编排、去重、排序与跨模块组合。
- `internal/model/`：统一模型、枚举和公共数据结构。
- `internal/runtimeinfo/`：读取 launchctl 实时状态。
- `internal/attribution/`：基于路径和 Info.plist 证据关联应用。
- `internal/signing/`：文件属性与代码签名检查。
- `internal/risk/`：纯函数风险规则、策略和例外处理。
- `internal/action/`：唯一允许修改系统持久化状态的业务层。
- `internal/monitor/`：快照、差异事件和历史记录。
- `internal/diagnostics/`：运行状态与 Unified Log 诊断。
- `internal/tui/`：分类、列表、详情和操作界面；复用 action 层，不复制业务规则。
- `internal/executil/`：外部命令白名单、无 Shell 执行、超时与输出上限。
- `schemas/`：对外 JSON Schema。
- `testdata/`：固定、脱敏、可复现的测试样本。
- `scripts/`：安装、校验和发布辅助脚本。
- `Formula/`：Homebrew HEAD Formula。
- `docs/`：产品、安全、架构、兼容性、安装和路线图文档。

## 常用命令

```bash
make verify             # 格式、vet、race test、ShellCheck、安装测试
make build              # 当前平台构建
go test ./internal/risk/...
go test ./internal/action/...
bash scripts/install_test.sh
make release-arm64
make release-amd64
```

提交前必须运行 `make verify`。若环境不是 macOS，至少完成完整 Go 测试和两个
Darwin 架构的交叉构建；真实扫描由 macOS CI 验证。

## 关键安全规则

- 所有状态修改必须进入 `internal/action`，其他层保持只读。
- 状态修改仅支持 launchd；BTM 与 cron 在 v1 保持只读。
- `/System/Library` 与 Apple 所有的任务永久禁止修改。
- 禁止 `sh -c`、动态命令路径和隐式 Shell；系统命令必须复用 `internal/executil` 的绝对路径白名单、超时和输出上限。
- Stoat 不得调用 `sudo`。系统级项目仅在调用进程已经是 root 且目录、文件所有权和权限通过验证时操作。
- 状态操作必须完整执行：`Plan → Confirm → Backup → Apply → Verify → Audit`。任何关键步骤失败都应停止并在需要时回滚。
- 确认令牌必须绑定操作类型、项目 ID、launchd domain、配置摘要和观测状态；任一内容变化后令牌失效。
- 备份与状态目录必须为当前主体私有，拒绝符号链接、非普通文件、宽松权限及不可信目录祖先。
- `remove` 只移除精确匹配的 launchd 配置并保留恢复材料。
- `uninstall` 只接受证据明确且直接位于 `/Applications` 或 `~/Applications` 的
  `.app`；绑定 `Info.plist` 摘要并移动到无冲突的废纸篓路径。
- 恢复前验证备份和目标状态，不覆盖后来创建的内容，不把已被篡改的备份当作可信输入。
- 风险结果只能表达复核优先级，禁止输出“安全”“病毒”“恶意软件”等无证据结论。
- 部分扫描失败必须显式返回 warning；不完整扫描不得推进监控基线。
- 安装器只使用 HTTPS，校验 SHA-256，限制归档成员，拒绝符号链接和路径穿越，原子替换二进制且不修改用户 Shell 配置。

任何扩大删除范围、归属匹配或权限边界的改动，都必须逐分支审查，并增加失败与回滚测试。

## 架构规则

- Collector 只采集，Parser 只解释，Risk 只评分，Action 只执行受控修改，TUI 只负责交互。
- 新 macOS 命令必须集中封装并可替换测试；测试不得依赖真实授权弹窗或真实系统修改。
- Parser 对未知字段向前兼容，对畸形、过大、非普通文件和符号链接输入失败关闭。
- 领域模型变化必须同步 JSON 输出、Schema、fixture、快照兼容性和架构文档。
- 风险规则保持确定性和纯函数特征；新增规则需要稳定 ID、分数、证据和 table-driven test。
- 错误需保留操作、来源和路径上下文；允许降级时使用 warning，不能静默吞错。
- 不为小功能引入新的外部依赖；确有必要时在 PR 中说明标准库或现有依赖为何不足。

## 交互规则

- 默认 TUI 保持“分类 → 列表 → 详情 → 操作”的层级，不恢复首次启动全部混排。
- `Esc` 逐级返回，`Enter` 进入，`a` 打开操作菜单；新增快捷键不得与现有行为冲突。
- 扫描开始、失败、空结果和操作结果都必须有明确状态，不能让终端无反馈等待。
- 危险操作必须展示对象、影响、可恢复性和确认要求，不用颜色代替文字含义。
- CLI 与 TUI 必须复用同一操作语义；不能出现界面可绕过 CLI 安全协议的路径。

## 测试要求

- Parser：使用脱敏 fixture 覆盖正常、未知字段、畸形、超限和跨 macOS 版本输入。
- Action：覆盖成功、确认失效、权限错误、状态竞争、审计失败、验证失败、回滚失败、恢复冲突和内容篡改。
- Risk：使用 table-driven test 固定规则 ID、证据与分数，避免依赖当前机器状态。
- TUI：测试分类导航、返回层级、加载状态、操作确认和执行后重扫。
- Installer：离线测试版本解析、架构选择、校验失败、恶意归档、原子替换和自定义目录。
- 修改并发、存储或缓存逻辑时运行 race test；修改 Formula、Shell 或 workflow
  时分别运行 Ruby 语法、ShellCheck 与 actionlint。

测试禁止修改真实 launchd、crontab、BTM 或 `/Applications`。需要系统行为时使用临时目录、fixture 和可注入 runner。

## 工作与提交

- 从最新 `main` 创建 `agent/<description>` 分支，不直接提交到 `main`。
- 修改前检查工作树；只提交本任务文件，不覆盖或清理用户的无关改动。
- Commit 保持单一职责且不添加 AI attribution trailer。
- PR 说明必须写清改动、原因、用户影响和验证结果；涉及安全边界时同时列出非目标。
- 代码注释解释“为什么”，不要复述语句本身；文档中的命令必须与当前 CLI 实际行为一致。
- 修复根因，不以放宽校验、忽略错误、增加无边界重试或兜底删除来掩盖问题。

## 发布

- 根目录 `VERSION` 是稳定版本来源；仅在明确发布新版本时修改。
- `VERSION` 合并到 `main` 后，Release workflow 会创建不可移动 Tag、构建双架构归档、生成校验和与 Sigstore bundle。
- 不手动移动既有 Tag，不覆盖 Release 资产，不在 CI 未通过时发布。
- 发布前核对版本、CHANGELOG、安装脚本、Formula 和文档示例一致。
- Apple Developer ID 签名与 notarization 尚未启用；不得把 Sigstore 描述为 Gatekeeper 公证的替代品。
- `stoat.lighting.pub`、国内镜像和稳定 Homebrew Tap 未实际上线前，文档必须明确标记状态。

## GitHub 操作

- 读取 Issue 或 PR 的最新正文、评论、状态与检查结果后再回复、关闭或合并。
- PR 默认保持 Draft，等待维护者确认；用户明确授权合并后才转为 Ready 并合并。
- CI 失败时读取真实 job 日志定位根因，不能仅凭工作流状态猜测。
- 合并后核对 `main` CI；若修改 `VERSION`，还需确认 Tag、Release 与全部资产真实存在。
