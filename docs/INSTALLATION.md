# 安装与升级

## 当前状态

仓库已公开。GitHub 一键安装依赖稳定版 Release；发布工作流会根据根目录 `VERSION` 自动创建 Tag 和 Release。

## 唯一安装方式

```bash
curl -fsSL https://raw.githubusercontent.com/wuuJiawei/stoat/main/scripts/install.sh | sh
```

固定版本，避免 `latest` 随时间变化：

```bash
curl -fsSL https://raw.githubusercontent.com/wuuJiawei/stoat/main/scripts/install.sh | \
  sh -s -- --version v1.2.0
```

安装器默认写入 `~/.local/bin/stoat`，不会调用 `sudo` 或修改 Shell 配置。自定义目录：

```bash
curl -fsSL https://raw.githubusercontent.com/wuuJiawei/stoat/main/scripts/install.sh | \
  sh -s -- --install-dir /usr/local/bin
```

目标目录必须已对当前用户可写。

不提供自有域名、第三方镜像或 GitHub 代理安装入口。

## 升级与卸载

再次运行安装器即可原子替换现有二进制。卸载默认安装：

```bash
rm ~/.local/bin/stoat
```

用户数据位于 `~/Library/Application Support/Stoat`，卸载二进制不会自动删除备份、审计或监控历史。

## 供应链校验

安装器对归档执行 SHA-256 校验，并验证归档只能包含 `stoat`、`README.md` 和 `LICENSE`。需要更强验证时，下载对应 `.sigstore.json` 后使用 README 中的 `cosign verify-blob` 命令。
