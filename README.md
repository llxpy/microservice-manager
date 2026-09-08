<div align="center">

```
  __  ___                 ____ __  __
 |  \/  |_   _ _ __ ___  / ___|  \/  | __ _ _ __   __ _  __ _  ___
 | |\/| | | | | '__/ _ \| |   | |\/| |/ ` | '_ \ / _` |/ _` |/ __|
 | |  | | |_| | | |  __/ |___| |  | | (_| | | | | (_| | (_| |\__ \
 |_|  |_|\__,_|_|  \___|\____|_|  |_|\__,_|_| |_|\__,_|\__, ||___/
                                                       |___/
```

### The lightweight butler for your local services

**One exe · ~50MB RAM · Java, Python, Node, Go — managed in one place.**

[Download](#-quick-start) · [中文文档](README.zh-CN.md)

![platform](https://img.shields.io/badge/platform-Windows%2010%2F11-blue)
![go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)
![memory](https://img.shields.io/badge/memory-~50MB-green)
![cgo](https://img.shields.io/badge/CGO-disabled-success)
![license](https://img.shields.io/badge/license-MIT-yellow)

</div>

---

## 🎯 Why does this exist?

If you develop microservices locally, you know the pain:

- Opening **IntelliJ IDEA just to run a few jars** costs you 1–2 GB of RAM.
- Starting 10 services means clicking Run 10 times — and one IDE crash kills them all.
- Logs are buried in a dozen editor tabs.
- Nobody knows *which* service is eating your CPU right now.
- After changing code you rebuild, restart, and re-check everything by hand.

**Micro-Manager replaces that workflow with a single web page.**

| | IntelliJ IDEA | Micro-Manager |
|---|---|---|
| Memory | 1–2 GB | **~50 MB** |
| Start 10 services | 10 clicks, tied to IDE | Group start, crash auto-restart |
| Logs | buried in tabs | xterm.js drawer + files on disk |
| Metrics / health | ✗ | CPU / memory curves + actuator health |
| Rebuild & rediscover | manual | one click, auto-scan |

## ✨ What you get

- 🔍 **Auto discovery** — point it at a folder; Spring Boot jars (and Python / Node / Go projects) are found, named, grouped and port-detected automatically
- 🌐 **Polyglot** — any `main.py`, `package.json` or `go.mod` project becomes a managed service
- 🛡 **Crash guard** — dead process? Back in 5 seconds
- 📜 **Live logs** — xterm.js in the browser, rotated files on disk
- 📈 **Metrics** — CPU / memory / threads curves, actuator health, port probing
- 🔨 **In-panel build** — `mvn package` with live output, auto re-scan when done
- 🖥 **System process monitor** — see everything running, kill what you don't need
- 📦 **Single binary** — frontend embedded, SQLite inside, zero install

## 🚀 Quick start

**No Go toolchain needed:**

1. Grab `dist/micro-manager.exe` from this repo
2. Double-click it
3. Open <http://localhost:9090> — point the scan box at your projects folder, done

<details>
<summary>Build from source</summary>

```bash
cd web && npm install && npm run build && cd ..
go build -ldflags "-s -w" -o micro-manager.exe .
```

</details>

## ⚠️ Honest limitations

- **Windows-first.** Process tree kill and auto-restart rely on `taskkill`; on Linux you'd want different signals.
- **No authentication.** Built for localhost / trusted LAN only. Don't expose it to the internet.
- **Force-kill semantics.** "Stop" uses `taskkill /F` — no graceful shutdown hooks.
- **Single machine.** No clustering, no remote agents.
- **Discovery is heuristic.** Exotic layouts (non-standard entry files, monorepo nesting) may need the manual "Add service" dialog.

## 🌏 Documentation

- [中文文档（完整使用教程）](README.zh-CN.md)

## 📄 License

MIT
