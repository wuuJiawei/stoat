<!-- markdownlint-disable-file MD013 MD033 MD041 -->

<div align="center">
  <h1>Stoat</h1>
  <p><em>看清并管理每一个在 Mac 上自动运行的项目。</em></p>
</div>

<p align="center">
  <a href="https://github.com/wuuJiawei/stoat/releases"><img src="https://img.shields.io/github/v/release/wuuJiawei/stoat?style=flat-square" alt="Release"></a>
  <a href="https://github.com/wuuJiawei/stoat/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/wuuJiawei/stoat/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/wuuJiawei/stoat/blob/main/go.mod"><img src="https://img.shields.io/github/go-mod/go-version/wuuJiawei/stoat?style=flat-square" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/wuuJiawei/stoat?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/macOS-13%2B-black?style=flat-square&logo=apple" alt="macOS 13+">
</p>

Stoat 是一个安全优先的 macOS 持久化检查与管理工具。它把登录项、`launchd`、后台任务和定时任务放进统一视图，解释项目从哪里来、当前是否运行、为什么值得关注，并在安全边界内提供停用、启用、隔离、移除与恢复能力。

> Stoat 提供复核线索，不是恶意软件检测器，也不会把“高风险”直接判定为“病毒”。

## 功能

- **统一发现**：扫描 Login Items、Background Task Management、LaunchAgents、LaunchDaemons 和 cron。
- **清晰归属**：结合 `.app` 路径、`Info.plist`、签名、文件属性和 `launchctl` 状态解释来源。
- **交互管理**：按“分类 → 列表 → 详情 → 操作”浏览，支持停用、启用、隔离、移除启动项和卸载已确认归属的应用。
- **持续观察**：保存快照、比较配置变化、记录历史事件，并结合 Unified Log 生成诊断信息。
- **安全操作**：强确认、私有备份、操作后验证、审计、失败回滚；应用只移动到废纸篓。
- **自动化友好**：提供表格、JSON、CSV 和 JSON 事件流，支持 Intel 与 Apple Silicon。

## 快速开始

### 安装

```bash
curl -fsSL https://raw.githubusercontent.com/wuuJiawei/stoat/main/scripts/install.sh | sh
```

安装器会识别 `arm64` / `amd64`，校验 SHA-256，并原子安装到 `~/.local/bin/stoat`。如果该目录不在 `PATH`：

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

### 运行

```bash
stoat
```

进入交互界面后：

- 使用方向键或 `1`–`5` 选择分类。
- 按 `Enter` 进入列表或查看详情，按 `a` 打开操作菜单。
- 按 `Esc` 逐级返回；删除和卸载需要输入指定确认词。

再次运行安装脚本即可升级，无需先卸载。

## 常用命令

```bash
# 查看
stoat startup                    # 登录与开机启动项
stoat scheduled                  # 定时任务
stoat background                 # 后台项目
stoat suspicious                 # 需要优先复核的项目
stoat inspect <id-or-label>      # 查看完整详情

# 管理：首次执行输出计划和一次性确认令牌
stoat disable <id-or-label>
stoat disable <id-or-label> --confirm <token>
stoat enable <id-or-label>
stoat quarantine <id-or-label>
stoat remove <id-or-label>       # 移除启动配置，保留可恢复备份
stoat uninstall <id-or-label>    # 将已归属应用移动到废纸篓
stoat restore <operation-id>

# 观察与导出
stoat scan --json
stoat snapshot --output before.json
stoat diff --json before.json after.json
stoat watch --interval 30s
stoat changes --limit 50
stoat diagnose <id-or-label> --last 1h
stoat export --format csv --output stoat-report.csv
```

运行 `stoat --help` 查看完整参数。

## 安全设计

Stoat 会读取多种 macOS 持久化来源，但不会对所有来源开放修改：

- 扫描默认只读；状态修改仅支持非 Apple 的 `launchd` 项。
- 不修改 BTM 私有数据库，不重写 crontab，不操作 `/System/Library` 项。
- 不调用 `sudo`，不通过 Shell 执行系统命令；系统级操作要求进程本身已是 root。
- 每次操作都绑定项目 ID、配置摘要和当前运行状态；配置变化后旧令牌自动失效。
- 修改前创建受保护备份，随后验证结果、写入审计；失败时恢复原配置与运行状态。
- 卸载只处理证据明确、位于 `/Applications` 或 `~/Applications` 的顶层 `.app`，不猜测或删除缓存、配置、账号及其他用户数据。
- 外部命令、文件大小、输出和整体扫描都有边界与超时；单一来源失败会产生 warning，不会隐藏其他结果。

安全模型与恢复流程见 [SECURITY.md](SECURITY.md) 和 [docs/SAFE_ACTIONS.md](docs/SAFE_ACTIONS.md)。

## 其他安装方式

### 从源码构建

```bash
git clone https://github.com/wuuJiawei/stoat.git
cd stoat
make verify
make build
./bin/stoat
```

需要 Go 1.25+。

### Homebrew HEAD（实验性）

```bash
brew tap wuuJiawei/stoat https://github.com/wuuJiawei/stoat.git
brew install --HEAD stoat
brew services start stoat
```

稳定版自有 Tap 与 `homebrew/core` 尚未发布，进度见 [Homebrew 说明](docs/HOMEBREW.md)。`stoat.lighting.pub` 和国内镜像的规划见 [安装文档](docs/INSTALLATION.md)。

## 兼容性与数据

- 目标系统：macOS 13+；CI 在 macOS 14 / 15 验证。
- 架构：Apple Silicon (`arm64`) 与 Intel (`amd64`)。
- 私有状态：`~/Library/Application Support/Stoat`。
- 快照 diff 关注持久化配置、签名、归属和禁用状态，不把 PID 或短暂运行状态变化视为配置变化。
- `sfltool dumpbtm` 与 `launchctl` 的部分诊断输出不是稳定公共格式，解析器只读取已知字段并保留未知字段兼容性。

## 文档

- [项目定位](docs/PROJECT.md)
- [架构说明](docs/ARCHITECTURE.md)
- [风险策略](docs/RISK_POLICY.md)
- [监控与变更历史](docs/MONITORING.md)
- [兼容性](docs/COMPATIBILITY.md)
- [路线图](docs/ROADMAP.md)
- [贡献指南](CONTRIBUTING.md)

## 致谢

特别感谢 [tw93/Mole](https://github.com/tw93/Mole)。Stoat 在终端优先的产品表达、TUI 交互、安全默认值和开源维护方式上受到 Mole 启发。

Mole 专注于 macOS 清理、卸载与系统维护；Stoat 专注于持久化项目的发现、解释和受控管理。两者解决的问题不同，Stoat 的代码、领域模型与安全操作协议均为独立实现。

也感谢 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 及所有参与反馈、测试和贡献的开发者。

## License

[MIT](LICENSE) © Stoat contributors
