<div align="center">

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:2563eb,100:7c3aed&height=200&section=header&text=Micro%20Manager&fontSize=68&fontColor=ffffff&desc=One%20exe%20%C2%B7%20~50MB%20%C2%B7%20Java%20%C2%B7%20Python%20%C2%B7%20Node%20%C2%B7%20Go&descSize=17&descAlignY=72" width="100%" />

**Stop paying 2GB of RAM just to run a few jars.**

[![platform](https://img.shields.io/badge/platform-Windows%2010%2F11-blue?logo=windows95)](#)
[![go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](#)
[![vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](#)
[![memory](https://img.shields.io/badge/RAM-~50MB-16a34a)](#)
[![cgo](https://img.shields.io/badge/CGO-disabled-16a34a)](#)
[![license](https://img.shields.io/badge/license-MIT-facc15)](#)

[✨ Features](#-what-you-get) · [🚀 Quick Start](#-quick-start) · [🔨 Build](#-build-inside-the-panel) · [⚠️ Limitations](#%EF%B8%8F-honest-limitations) · [中文文档](README.zh-CN.md)

</div>

<br />

<pre align="center">
 __  __ _                __  __
|  \/  (_) ___ _ __ ___ |  \/  | __ _ _ __   __ _  __ _  ___ _ __
| |\/| | |/ __| '__/ _ \| |\/| |/ _` | '_ \ / _` |/ _` |/ _ \ '__|
| |  | | | (__| | | (_) | |  | | (_| | | | | (_| | (_| |  __/ |
|_|  |_|_|\___|_|  \___/|_|  |_|\__,_|_| |_|\__,_|\__, |\___|_|
                                                  |___/
</pre>

## 🎯 The problem it solves

You built microservices locally. Then your daily life became:

> Open IDEA (2GB RAM) → click Run 10 times → dig logs out of 12 editor tabs → guess which service is eating CPU → change code → rebuild by hand → repeat.

**Micro-Manager collapses all of that into one browser tab.**

| | IntelliJ IDEA | Micro-Manager |
|:---|:---|:---|
| Memory | 1–2 GB | **~50 MB** |
| Starting 10 services | 10 clicks, dies with the IDE | group start + crash auto-restart |
| Logs | buried in tabs | xterm.js drawer + rotated files on disk |
| CPU / memory / health | ✗ | live curves + actuator health |
| Rebuild & rediscover | manual | one click, auto-scan |

## ✨ What you get

| | |
|:---|:---|
| 🔍 **Auto discovery** | Point at a folder. Spring Boot jars & Python / Node / Go projects get found, named, grouped, port-detected |
| 🌐 **Polyglot** | Any `main.py` / `package.json` / `go.mod` project becomes a managed service |
| 🛡 **Crash guard** | Process dies → back online in 5s |
| 📜 **Live logs** | xterm.js in-browser terminal + rotated files on disk |
| 📈 **Metrics & health** | CPU / memory / threads curves, actuator health, port probing |
| 🔨 **In-panel build** | `mvn package` with live output, auto re-scan when done |
| 🖥 **System processes** | See every process on the machine, kill the noisy ones |
| 📦 **Single binary** | Frontend embedded, SQLite inside, zero install |

## 🚀 Quick start

**No Go, no Node, no setup:**

```txt
1. Download  dist/micro-manager.exe  from this repo
2. Double-click it
3. Open  http://localhost:9090
4. Point the scan box at your projects folder — everything appears
```

<details>
<summary><b>Build from source</b></summary>

```bash
git clone https://github.com/llxpy/microservice-manager.git
cd microservice-manager
cd web && npm install && npm run build && cd ..
go build -ldflags "-s -w" -o micro-manager.exe .
```

</details>

## ⚠️ Honest limitations

- **Windows-first** — process-tree kill & crash guard rely on `taskkill`
- **No auth** — localhost / trusted LAN only, never expose it
- **Force stop** — "stop" is `taskkill /F`, no graceful shutdown hooks
- **Single machine** — no cluster, no remote agents
- **Heuristic discovery** — exotic layouts may need the manual "Add service" dialog

<br />

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:2563eb,100:7c3aed&height=100&section=footer" width="100%" />
