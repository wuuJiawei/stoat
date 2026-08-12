# 安装与升级

## 当前状态

仓库仍为 Private，以下公开一键安装地址只有在 `v1.0.0` Release 可匿名下载后才对普通用户可用。已授权的 GitHub 用户仍可使用私有 HEAD Tap；详见 [HOMEBREW.md](HOMEBREW.md)。

## GitHub 一键安装

公开发布后：

```bash
curl -fsSL https://raw.githubusercontent.com/wuuJiawei/stoat/main/scripts/install.sh | sh
```

固定版本，避免 `latest` 随时间变化：

```bash
curl -fsSL https://raw.githubusercontent.com/wuuJiawei/stoat/main/scripts/install.sh | \
  sh -s -- --version v1.0.0
```

安装器默认写入 `~/.local/bin/stoat`，不会调用 `sudo` 或修改 Shell 配置。自定义目录：

```bash
curl -fsSL https://raw.githubusercontent.com/wuuJiawei/stoat/main/scripts/install.sh | \
  sh -s -- --install-dir /usr/local/bin
```

目标目录必须已对当前用户可写。

## lighting.pub 一键安装

域名部署完成后：

```bash
curl -fsSL https://stoat.lighting.pub/install.sh | sh
```

站点需按以下路径发布不可变文件：

```text
/install.sh
/install-cn.sh
/releases/latest.txt
/releases/v1.0.0/checksums.txt
/releases/v1.0.0/stoat-v1.0.0-darwin-arm64.tar.gz
/releases/v1.0.0/stoat-v1.0.0-darwin-amd64.tar.gz
```

使用域名分发：

```bash
curl -fsSL https://stoat.lighting.pub/install.sh | \
  sh -s -- \
    --metadata-base https://stoat.lighting.pub/releases \
    --download-base https://stoat.lighting.pub/releases
```

## 国内 GitHub 加速镜像

推荐由 `stoat.lighting.pub` 同步 GitHub Release，提供安装脚本、校验和与归档：

```bash
curl -fsSL https://stoat.lighting.pub/install-cn.sh | sh
```

如果不在 `lighting.pub` 保存归档，可通过环境变量指定经过评估的 HTTPS GitHub 代理：

```bash
curl -fsSL https://stoat.lighting.pub/install-cn.sh | \
  STOAT_GITHUB_PROXY=https://your-github-proxy.example/ sh
```

也可直接指定固定版本和代理：

```bash
curl -fsSL https://stoat.lighting.pub/install.sh | \
  STOAT_GITHUB_PROXY=https://your-github-proxy.example/ \
  sh -s -- --version v1.0.0
```

代理 URL 会作为前缀拼接到完整 GitHub Release URL。第三方代理可能记录请求或替换内容；安装器会校验 SHA-256，但应让 `checksums.txt` 来自你控制的 `stoat.lighting.pub`。

## 升级与卸载

再次运行安装器即可原子替换现有二进制。卸载默认安装：

```bash
rm ~/.local/bin/stoat
```

Homebrew 安装使用：

```bash
brew uninstall stoat
brew untap wuuJiawei/stoat
```

用户数据位于 `~/Library/Application Support/Stoat`，卸载二进制不会自动删除备份、审计或监控历史。

## 供应链校验

安装器对归档执行 SHA-256 校验，并验证归档只能包含 `stoat`、`README.md` 和 `LICENSE`。需要更强验证时，下载对应 `.sigstore.json` 后使用 README 中的 `cosign verify-blob` 命令。
