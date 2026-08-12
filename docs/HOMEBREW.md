# Homebrew 发布状态

## 当前可用：私有 HEAD Tap

仓库内 Formula 可供已获得 Private 仓库权限的用户从源码安装：

```bash
brew tap wuuJiawei/stoat git@github.com:wuuJiawei/stoat.git
brew install --HEAD stoat
```

SSH 方式要求本机 GitHub SSH Key 已有仓库读取权限。`brew services start stoat` 可启动持续监控。

## 公开自有 Tap 仍需准备

1. 将 Stoat 源码仓库公开，或提供无需鉴权的不可变源码归档。
2. 创建公开仓库 `wuuJiawei/homebrew-stoat`，在 `Formula/stoat.rb` 发布稳定版本 URL 与 SHA-256。
3. 创建稳定 Git tag 和 GitHub Release；当前私有 Release 无法供普通 Homebrew 用户匿名下载。
4. 如分发预编译二进制，准备 Apple Developer ID 签名与公证；当前 Sigstore 签名不能替代 Apple 公证。

自有 Tap 不需要 Homebrew 官方审核，但由项目自行维护和修复。

## 暂不能直接进入 homebrew/core

当前源码已达到 `v1.0.0` 功能边界，但仓库仍为 Private，也尚未形成可匿名下载的稳定 Release。Homebrew 官方要求可验证的不可变稳定源码、兼容许可证、在其 CI 矩阵构建测试，并要求新项目具备一定知名度（例如至少 75 stars、30 forks 或 30 watchers）。现阶段不满足直接提交条件。

发布状态统一记录在 [ROADMAP.md](ROADMAP.md)，其他安装方式见 [INSTALLATION.md](INSTALLATION.md)。

官方依据：[Acceptable Formulae](https://docs.brew.sh/Acceptable-Formulae)、[How to Create and Maintain a Tap](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap)。
