# Stoat

Stoat 是一个安全优先的 macOS 持久化任务检查器，用于回答：哪些程序会在登录、开机、定时或后台自动运行，以及哪些项目值得人工复核。

当前为 `v0.5.0`，最低目标系统为 macOS 13。

## 已实现

- 扫描 Login Items / Background Task Management、LaunchAgents、LaunchDaemons、用户 crontab 和 `/etc/crontab`
- 统一数据模型与 Startup / Scheduled / Background 多分类
- XML、binary plist 通过系统 `plutil` 安全转换后解析
- 检查可执行文件是否存在、权限和代码签名
- 可解释的风险评分，不输出“病毒/安全”结论
- Bubble Tea TUI、表格输出和 JSON 输出
- 默认不扫描 Apple `/System/Library` 基准项，可用 `--system` 显式加入
- 读取 launchctl 实时加载、运行、PID、退出码和禁用状态
- 基于 `.app` 路径与 `Info.plist` 证据关联所属应用，不根据 label 猜测
- JSON / CSV 安全导出；文件以 `0600` 原子写入且默认拒绝覆盖
- macOS 13 / 14 / 15 脱敏 BTM fixture 与解析性能基准
- 扫描快照与新增、删除、配置变更对比；忽略 PID 等易变运行字段
- 每条风险 finding 具有稳定规则 ID、分数、证据和可审计的屏蔽状态
- 严格 JSON 风险例外策略，仅允许按确定 item ID 屏蔽指定正向规则
- Homebrew HEAD Formula、双架构 Release、SHA-256 校验和与 Sigstore 无密钥签名
- launchd 项停用、隔离与恢复；BTM 和 cron 保持只读
- 配置摘要绑定的双阶段确认令牌，配置变化后旧令牌自动失效
- 操作前备份、失败回滚、恢复验证与私有操作审计

## 安全边界

- 状态修改只支持 launchd；不修改 BTM 私有数据库或重写 crontab
- 不调用 `sudo`；系统级项目只允许已是 root 的进程操作
- `/System/Library` 永久禁止修改，配置路径、目录符号链接和权限均需验证
- 停用或隔离前保存受保护备份，失败时回滚并记录结果
- 不调用 Shell：参数直接传给 `exec.CommandContext`
- 系统命令使用绝对路径白名单，防止 `PATH` 劫持
- 外部命令 3 秒超时、输出大小受限，整体扫描 45 秒超时
- 跳过符号链接、设备文件及超大配置文件
- 单个来源失败仅产生 warning，其他结果仍可使用

详见 [SECURITY.md](SECURITY.md)、[项目章程](docs/PROJECT.md) 和 [架构说明](docs/ARCHITECTURE.md)。

## 开发

要求 Go 1.25+。

```bash
make verify
make build
./bin/stoat
```

## 使用

```bash
stoat
stoat scan
stoat scan --json
stoat startup
stoat scheduled
stoat background
stoat suspicious
stoat inspect <id-or-label>
stoat export --format json --output stoat-report.json
stoat export --format csv --output stoat-report.csv
stoat snapshot --output before.json
stoat diff --json before.json after.json
stoat scan --rules policy.json
stoat scan --system
stoat disable <id-or-label>
stoat disable <id-or-label> --confirm <token>
stoat quarantine <id-or-label>
stoat restore <operation-id>
stoat audit [operation-id]
```

`stoat suspicious` 展示 Attention 和 High 项；风险原因只是复核线索，不是恶意软件判定。

风险例外格式见 [docs/RISK_POLICY.md](docs/RISK_POLICY.md)，状态修改协议见 [docs/SAFE_ACTIONS.md](docs/SAFE_ACTIONS.md)。

## 安装

从当前私有仓库使用 Homebrew 构建安装：

```bash
brew tap wuuJiawei/stoat https://github.com/wuuJiawei/stoat
brew install --HEAD stoat
```

版本 Tag 会自动生成 Apple Silicon / Intel 压缩包、SHA-256 校验和和 Sigstore 无密钥签名包。当前没有 Apple Developer ID，因此 Release 二进制尚未进行 Apple 公证。

```bash
cosign verify-blob stoat-v0.5.0-darwin-arm64.tar.gz \
  --bundle stoat-v0.5.0-darwin-arm64.tar.gz.sigstore.json \
  --certificate-identity-regexp='^https://github.com/wuuJiawei/stoat/.github/workflows/release.yml@refs/tags/v[0-9].*$' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com'
```

## 项目结构

```text
cmd/stoat              CLI 入口
internal/app           扫描编排、去重、排序
internal/collector     macOS 数据采集
internal/executil      外部命令安全边界
internal/model         统一领域模型
internal/parser        launchd / cron / BTM 解析
internal/signing       文件属性与代码签名
internal/risk          纯函数规则引擎
internal/tui           Bubble Tea 只读界面
testdata               固定测试样本
```

## 当前限制

- `sfltool dumpbtm` 属于系统诊断输出，不是稳定公共 API；解析器忽略未知字段并保留兼容性，但仍需在不同 macOS 版本建立 fixture 回归库。
- `launchctl` 的诊断文本不是稳定公共数据格式；解析器仅读取已知字段并保持未知字段兼容。
- 无法从可执行路径或来源 Bundle ID 建立证据链的项目会明确显示为 `Unattributed`。
- 快照 diff 关注持久化配置、签名、归属、文件属性和禁用状态，不将 PID、运行/停止切换视为配置变更。
- BTM 和 cron 修改仍不开放；前者依赖私有数据库，后者需安全保留整份 crontab 的语义和并发修改。

## 设计来源

工程组织参考 [tw93/Mole](https://github.com/tw93/Mole) 的 Go + Bubble Tea 组件化方式、质量检查与超时约束；Stoat 未复制 Mole 源码，并针对持久化检查采用独立领域模型与只读安全边界。
