<div align="center">

```
  __  ___                 ____ __  __
 |  \/  |_   _ _ __ ___  / ___|  \/  | __ _ _ __   __ _  __ _  ___
 | |\/| | | | | '__/ _ \| |   | |\/| |/ ` | '_ \ / _` |/ _` |/ __|
 | |  | | |_| | | |  __/ |___| |  | | (_| | | | | (_| | (_| |\__ \
 |_|  |_|\__,_|_|  \___|\____|_|  |_|\__,_|_| |_|\__,_|\__, ||___/
                                                       |___/
```

### 你本地服务的轻量管家

**一个 exe · 常驻约 50MB · Java、Python、Node、Go 统一管理。**

[下载](#-快速开始) · [English](README.md)

![platform](https://img.shields.io/badge/platform-Windows%2010%2F11-blue)
![go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)
![memory](https://img.shields.io/badge/memory-~50MB-green)
![cgo](https://img.shields.io/badge/CGO-disabled-success)
![license](https://img.shields.io/badge/license-MIT-yellow)

</div>

---

## 🎯 它解决什么问题

本地开发微服务的日常：

- 为了跑几个 jar 打开 **IDEA，白白吃掉 1~2GB 内存**
- 启动 10 个服务点 10 次 Run，IDE 一崩全没
- 日志埋在十几个编辑器 Tab 里来回切
- 没人知道此刻是哪个服务在吃 CPU
- 改完代码：手动重新打包 → 重启 → 逐个检查

**微服务轻量管家把这套流程压成一个网页。**

| | IntelliJ IDEA | 轻量管家 |
|---|---|---|
| 内存占用 | 1~2 GB | **~50 MB** |
| 启动 10 个服务 | 逐个点，绑死 IDE | 分组批量启动 + 崩溃自动守护 |
| 日志 | 十几个 Tab | 网页 xterm 终端 + 文件落盘 |
| 资源/健康监控 | ✗ | CPU/内存曲线 + 健康检查 |
| 重新打包与发现 | 全手动 | 一键构建 + 自动扫描 |

## ✨ 核心能力

- 🔍 **自动发现** — 指一个目录：Spring Boot jar（以及 Python / Node / Go 项目）自动识别、命名、按项目分组、自动探测端口
- 🌐 **多语言** — 任何含 `main.py`、`package.json`、`go.mod` 的项目都能成为受管服务
- 🛡 **崩溃守护** — 进程挂了 5 秒内自动拉起
- 📜 **实时日志** — 浏览器内 xterm 终端，磁盘按大小切分落盘
- 📈 **资源监控** — CPU/内存/线程曲线、Actuator 健康、端口探测
- 🔨 **面板内构建** — `mvn package` 实时日志，完成后自动扫描新服务
- 🖥 **系统进程监控** — 看到整机所有进程，随手结束不需要的
- 📦 **单一可执行文件** — 前端已嵌入，SQLite 内置，零安装

## 🚀 快速开始

**不需要 Go 环境：**

1. 下载本仓库的 `dist/micro-manager.exe`
2. 双击运行
3. 浏览器打开 <http://localhost:9090>，把扫描目录指向你的项目文件夹，完事

<details>
<summary>从源码构建</summary>

```bash
cd web && npm install && npm run build && cd ..
go build -ldflags "-s -w" -o micro-manager.exe .
```

</details>

## ⚠️ 诚实的缺点

- **Windows 优先。** 进程树清理与守护依赖 `taskkill`，Linux 下需要改用信号方案
- **无鉴权。** 只适合本机和可信局域网，不要暴露公网
- **强制停止。** 「停止」使用 `taskkill /F`，不会触发优雅停机钩子
- **单机。** 没有集群、没有远程节点
- **发现是启发式的。** 特殊工程结构（非标准入口文件、深层嵌套）可能需要用「添加服务」手动注册

## 📄 License

MIT
