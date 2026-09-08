<div align="center">

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:2563eb,100:7c3aed&height=200&section=header&text=Micro%20Manager&fontSize=68&fontColor=ffffff&desc=%E4%B8%80%E4%B8%AAexe%20%C2%B7%20%E5%B8%B8%E9%A9%BB%2050MB%20%C2%B7%20Java%20%C2%B7%20Python%20%C2%B7%20Node%20%C2%B7%20Go&descSize=17&descAlignY=72" width="100%" />

**别再为了跑几个 jar 开着 2GB 内存的 IDEA 了。**

[![platform](https://img.shields.io/badge/platform-Windows%2010%2F11-blue?logo=windows95)](#)
[![go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](#)
[![vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](#)
[![memory](https://img.shields.io/badge/RAM-~50MB-16a34a)](#)
[![cgo](https://img.shields.io/badge/CGO-disabled-16a34a)](#)
[![license](https://img.shields.io/badge/license-MIT-facc15)](#)

[✨ 核心能力](#-核心能力) · [🚀 快速开始](#-快速开始) · [🔨 构建](#-面板内构建) · [⚠️ 缺点](#%EF%B8%8F-诚实的缺点) · [English](README.md)

</div>

<br />

## 🎯 它解决什么问题

本地开发微服务的日常：

> 打开 IDEA（2GB 内存）→ 点 10 次 Run → 在 12 个编辑器 Tab 里翻日志 → 猜是哪个服务在吃 CPU → 改代码 → 手动重新打包 → 循环。

**微服务轻量管家把这一切压进一个浏览器标签页。**

| | IntelliJ IDEA | 轻量管家 |
|:---|:---|:---|
| 内存占用 | 1~2 GB | **~50 MB** |
| 启动 10 个服务 | 点 10 次，绑死 IDE | 分组批量启动 + 崩溃自动守护 |
| 日志 | 埋在 Tab 里 | xterm 网页终端 + 磁盘切分落盘 |
| CPU / 内存 / 健康 | ✗ | 实时曲线 + 健康检查 |
| 重新打包与发现 | 全手动 | 一键构建 + 自动扫描 |

## ✨ 核心能力

| | |
|:---|:---|
| 🔍 **自动发现** | 指一个目录：Spring Boot jar 和 Python / Node / Go 项目自动识别、命名、按项目分组、探测端口 |
| 🌐 **多语言** | 任何含 `main.py` / `package.json` / `go.mod` 的项目都能成为受管服务 |
| 🛡 **崩溃守护** | 进程挂了 5 秒内自动拉起 |
| 📜 **实时日志** | 浏览器内 xterm 终端 + 磁盘按大小切分落盘 |
| 📈 **资源与健康** | CPU / 内存 / 线程曲线、Actuator 健康、端口探测 |
| 🔨 **面板内构建** | `mvn package` 实时日志，完成后自动扫描新服务 |
| 🖥 **系统进程** | 看到整机所有进程，随手结束不需要的 |
| 🖅 **托盘常驻** | 无黑窗口，托盘图标一键打开面板，优雅退出 |
| 📦 **单一可执行文件** | 前端已嵌入，SQLite 内置，零安装 |

## 🚀 快速开始

**不需要 Go、不需要 Node、零配置：**

```txt
1. 下载本仓库的  dist/micro-manager.exe
2. 双击运行 —— 无黑窗口，托盘出现渐变图标，浏览器自动打开面板
3. 没弹？托盘右键「打开面板」，或访问  http://localhost:9090
4. 扫描目录指向你的项目文件夹 —— 服务全部自动出现
```

<details>
<summary><b>从源码构建</b></summary>

```bash
git clone https://github.com/llxpy/microservice-manager.git
cd microservice-manager
cd web && npm install && npm run build && cd ..
go build -ldflags "-s -w -H=windowsgui" -o micro-manager.exe .
```

</details>

<details>
<summary><b>托盘与日志</b></summary>

- 托盘左键菜单：**打开面板** / **退出**（退出不影响已启动的服务进程）
- 面板运行日志：`logs/manager.log`（启动 banner 也在里面）
- 端口被占用等启动错误会以系统弹窗提示

</details>

## ⚠️ 诚实的缺点

- **Windows 优先** — 进程树清理与崩溃守护依赖 `taskkill`
- **无鉴权** — 只适合本机 / 可信局域网，切勿暴露公网
- **强制停止** — 「停止」是 `taskkill /F`，不触发优雅停机钩子
- **单机** — 无集群、无远程节点
- **启发式发现** — 特殊工程结构可能需要「添加服务」手动注册

<br />

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:2563eb,100:7c3aed&height=100&section=footer" width="100%" />
