# Stoat 路线图与待办

本文件是进入 `v1.0.0` 的唯一功能清单。状态含义：

- `[x]` 已完成：代码、文档和自动化验证已进入仓库。
- `[ ]` Feature：依赖外部账号、公开资源或发布材料，保留到条件满足后完成。

## v1.0.0 稳定版

- [x] Login Items、BTM、launchd、cron 统一扫描与可解释风险结果。
- [x] launchd 停用、隔离、恢复、备份、审计和失败回滚。
- [x] 快照对比、持续变更监控、历史事件和 Unified Log 诊断。
- [x] macOS 14 / 15 CI 实机扫描，Darwin arm64 / amd64 构建，race test 和 vet。
- [x] GitHub Release 双架构压缩包、SHA-256 校验和和 Sigstore bundle。
- [x] 安全一键安装脚本：自动识别 Apple Silicon / Intel、校验 SHA-256、原子安装到用户目录。
- [x] 基于 `raw.githubusercontent.com` 和 GitHub Release 的唯一安装方案。
- [x] 安装脚本的离线集成测试、ShellCheck 和 Release 资产发布。
- [x] 安装、升级、卸载、PATH、私有仓库限制和供应链校验文档。

## 待完成 Feature

- [ ] **Apple Developer ID 签名与 notarization**：提供 Apple Developer 账号、证书及 CI Secret 后启用；Sigstore 不能替代 Gatekeeper 公证。
- [x] **公开仓库**：源码与安装脚本已可匿名访问。
- [x] **自动稳定版发布**：根目录 `VERSION` 变更合并到 `main` 后自动创建 Tag、Release 和双架构资产。

## v1.1.0 管理闭环

- [x] TUI 详情页提供停用、启用、隔离、删除启动项和卸载应用操作。
- [x] CLI 增加 `enable`、`remove`、`uninstall`，与 TUI 共用安全操作层。
- [x] 删除启动项保留可验证备份；卸载应用移动到废纸篓，两者均可恢复。
- [x] 状态变化检测、强确认、操作后重扫、失败回滚和故障注入测试。

## 发布原则

1. 官方安装入口固定为 `raw.githubusercontent.com`，安装包与校验和来自 GitHub Release。
2. 一键安装默认写入 `~/.local/bin`，不调用 `sudo`，不修改 Shell 配置。
3. 不新增自有域名、第三方镜像、GitHub 代理或 Homebrew 安装入口。
4. BTM、cron 和 Apple `/System/Library` 项在 v1 继续只读。
