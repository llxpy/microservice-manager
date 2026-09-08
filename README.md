<div align="center">

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:2563eb,100:7c3aed&height=200&section=header&text=Micro%20Manager&fontSize=68&fontColor=ffffff&desc=One%20exe%20%C2%B7%20~50MB%20%C2%B7%20Java%20%C2%B7%20Python%20%C2%B7%20Node%20%C2%B7%20Go&descSize=17&descAlignY=72" width="100%" />

**Stop paying 2GB of RAM just to run a few jars.**

[![platform](https://img.shields.io/badge/platform-Windows%2010%2F11-blue?logo=windows95)](#)
[![go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](#)
[![vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white)](#)
[![memory](https://img.shields.io/badge/RAM-~50MB-16a34a)](#)
[![cgo](https://img.shields.io/badge/CGO-disabled-16a34a)](#)
[![license](https://img.shields.io/badge/license-MIT-facc15)](#)

[鉁?Features](#-what-you-get) 路 [馃殌 Quick Start](#-quick-start) 路 [馃敤 Build](#-build-inside-the-panel) 路 [鈿狅笍 Limitations](#%EF%B8%8F-honest-limitations) 路 [涓枃鏂囨。](README.zh-CN.md)

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

## 馃幆 The problem it solves

You built microservices locally. Then your daily life became:

> Open IDEA (2GB RAM) 鈫?click Run 10 times 鈫?dig logs out of 12 editor tabs 鈫?guess which service is eating CPU 鈫?change code 鈫?rebuild by hand 鈫?repeat.

**Micro-Manager collapses all of that into one browser tab.**

| | IntelliJ IDEA | Micro-Manager |
|:---|:---|:---|
| Memory | 1鈥? GB | **~50 MB** |
| Starting 10 services | 10 clicks, dies with the IDE | group start + crash auto-restart |
| Logs | buried in tabs | xterm.js drawer + rotated files on disk |
| CPU / memory / health | 鉁?| live curves + actuator health |
| Rebuild & rediscover | manual | one click, auto-scan |

## 鉁?What you get

| | |
|:---|:---|
| 馃攳 **Auto discovery** | Point at a folder. Spring Boot jars & Python / Node / Go projects get found, named, grouped, port-detected |
| 馃寪 **Polyglot** | Any `main.py` / `package.json` / `go.mod` project becomes a managed service |
| 馃洝 **Crash guard** | Process dies 鈫?back online in 5s |
| 馃摐 **Live logs** | xterm.js in-browser terminal + rotated files on disk |
| 馃搱 **Metrics & health** | CPU / memory / threads curves, actuator health, port probing |
| 馃敤 **In-panel build** | `mvn package` with live output, auto re-scan when done |
| 馃枼 **System processes** | See every process on the machine, kill the noisy ones |
| 馃摝 **Single binary** | Frontend embedded, SQLite inside, zero install |

## 馃殌 Quick start

**No Go, no Node, no setup:**

```txt
1. Download  dist/micro-manager.exe  from this repo
2. Double-click it - a tray icon appears and the panel opens in your browser
3. No tray icon? Open  http://localhost:9090
4. Point the scan box at your projects folder 鈥?everything appears
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

## 鈿狅笍 Honest limitations

- **Windows-first** 鈥?process-tree kill & crash guard rely on `taskkill`
- **No auth** 鈥?localhost / trusted LAN only, never expose it
- **Force stop** 鈥?"stop" is `taskkill /F`, no graceful shutdown hooks
- **Single machine** 鈥?no cluster, no remote agents
- **Heuristic discovery** 鈥?exotic layouts may need the manual "Add service" dialog

<br />

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:2563eb,100:7c3aed&height=100&section=footer" width="100%" />
