# Stoat

Stoat 是一个只读的 macOS 持久化任务检查器，用于回答：哪些程序会在登录、开机、定时或后台自动运行，以及哪些项目值得人工复核。

当前为 `v0.1.0` 雏形，最低目标系统为 macOS 13。

## 已实现

- 扫描 Login Items / Background Task Management、LaunchAgents、LaunchDaemons、用户 crontab 和 `/etc/crontab`
- 统一数据模型与 Startup / Scheduled / Background 多分类
- XML、binary plist 通过系统 `plutil` 安全转换后解析
- 检查可执行文件是否存在、权限和代码签名
- 可解释的风险评分，不输出“病毒/安全”结论
- Bubble Tea TUI、表格输出和 JSON 输出
- 默认不扫描 Apple `/System/Library` 基准项，可用 `--system` 显式加入

## 安全边界

- 只读：没有删除、禁用、提权和配置写入代码
- 不调用 Shell：参数直接传给 `exec.CommandContext`
- 系统命令使用绝对路径白名单，防止 `PATH` 劫持
- 外部命令 3 秒超时、输出大小受限，整体扫描 45 秒超时
- 跳过符号链接、设备文件及超大配置文件
- 单个来源失败仅产生 warning，其他结果仍可使用

详见 [SECURITY.md](SECURITY.md)、[项目章程](docs/PROJECT.md) 和 [架构说明](docs/ARCHITECTURE.md)。

## 开发

要求 Go 1.24+。

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
stoat scan --system
```

`stoat suspicious` 展示 Attention 和 High 项；风险原因只是复核线索，不是恶意软件判定。

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
- launchctl 实时状态、App 归属关联和导出文件将在下一阶段补齐。
- V1 不接受任何“禁用/删除”功能；后续若增加，必须先设计快照、恢复和显式确认。

## 设计来源

工程组织参考 [tw93/Mole](https://github.com/tw93/Mole) 的 Go + Bubble Tea 组件化方式、质量检查与超时约束；Stoat 未复制 Mole 源码，并针对持久化检查采用独立领域模型与只读安全边界。
