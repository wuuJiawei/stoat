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
- [x] GitHub、`stoat.lighting.pub` 和可配置 GitHub 加速镜像的安装方案。
- [x] 安装脚本的离线集成测试、ShellCheck 和 Release 资产发布。
- [x] 安装、升级、卸载、PATH、私有仓库限制和供应链校验文档。

## 待完成 Feature

- [ ] **公开 Homebrew Tap**：将 Stoat 源码或不可变 Release 归档公开，创建公开的 `wuuJiawei/homebrew-stoat`，写入稳定版 URL 与 SHA-256。
- [ ] **homebrew/core**：仓库公开、稳定版本形成用户量并满足 Homebrew 接受标准后再提交。
- [ ] **`stoat.lighting.pub` 上线**：配置 DNS、HTTPS 和静态托管；发布 `install.sh`、`install-cn.sh`、`latest.txt`、`checksums.txt` 与 Release 归档。
- [ ] **国内镜像节点上线**：选择或自建可信镜像；镜像只分发归档，校验和仍优先从 `stoat.lighting.pub` 获取。
- [ ] **Apple Developer ID 签名与 notarization**：提供 Apple Developer 账号、证书及 CI Secret 后启用；Sigstore 不能替代 Gatekeeper 公证。
- [x] **公开仓库**：源码与安装脚本已可匿名访问。
- [x] **自动稳定版发布**：根目录 `VERSION` 变更合并到 `main` 后自动创建 Tag、Release 和双架构资产。

## 发布原则

1. 第三方镜像不可成为唯一信任源；安装器必须核对受信来源提供的 SHA-256。
2. 一键安装默认写入 `~/.local/bin`，不调用 `sudo`，不修改 Shell 配置。
3. 域名、镜像或 Homebrew 未真实可用前，文档必须明确标记为待上线。
4. BTM、cron 和 Apple `/System/Library` 项在 v1 继续只读。
