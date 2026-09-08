# Java 微服务轻量管家

单文件、零依赖的 Windows 本机 Java 服务管理面板。用 Go 编写，常驻内存约 **50MB**，替代 IDEA 来托管 `java -jar` 服务：

- 🔍 **自动发现**：扫描目录下所有 `*.jar`，自动解析 `application.yml` / `application.properties` 中的 `server.port`、`spring.application.name`
- ▶️ **一键启停**：启动 / 停止 / 重启，按分组批量操作，进程树整体退出（`taskkill /T`）
- 🛡️ **崩溃守护**：进程意外退出后 5s 自动重拉（可按服务关闭）
- 📜 **实时日志**：网页内 xterm.js 终端实时输出，同时落盘 `logs/<服务名>.log`（按大小切分，保留 3 份）
- 📈 **资源监控**：CPU / 内存 / 线程数实时曲线（ECharts），TCP 端口探测 + Actuator 健康检查
- 💾 **持久化**：服务定义存 SQLite（单文件），`services.yaml` 手动定义优先

> 明确不做：代码编辑、LSP、自动编译、Debug、热更新、鉴权。只做单机管家。

---

## 快速开始（使用已编译好的 exe）

1. 把 `micro-manager.exe` 放到任意目录，同目录放一份 `config.yaml`（可选，不放则用默认值）：

   ```yaml
   server:
     host: 0.0.0.0
     port: 9090        # 面板端口
   scanDir: D:\svc     # jar 扫描目录（递归）
   logDir: ./logs      # 日志落盘目录
   logMaxSizeMB: 50    # 单个日志文件上限
   metricsInterval: 3s # 资源采集间隔
   healthInterval: 10s # 健康检查间隔
   restartDelay: 5s    # 崩溃自动重启延迟
   stopTimeout: 10s    # 停止超时强杀时间
   ```

2. （可选）编辑 `services.yaml` 手动定义服务，**手动定义优先于自动发现**：

   ```yaml
   services:
     - name: gateway
       path: D:\svc\gateway.jar
       javaOpts: "-Xms96m -Xmx192m -XX:+UseSerialGC"
       port: 8080
       healthUrl: http://localhost:8080/actuator/health
       group: base          # 分组，用于批量启停
       autoRestart: true
   ```

3. 双击运行 `micro-manager.exe`，浏览器打开 **http://localhost:9090**。

   首次启动会自动扫描 `scanDir`，把所有 jar 登记进面板；之后每 30s 重扫一次，新 jar 自动出现（也可点「重新扫描」立即触发）。

## 从源码构建

依赖：Go 1.22+、Node 18+

```powershell
# 1. 构建前端（产物输出到 internal/webdist，被 go:embed 嵌入）
cd web
npm install
npm run build
cd ..

# 2. 构建后端（纯 Go，无 CGO）
go build -ldflags "-s -w" -o micro-manager.exe .
```

## 页面使用

| 操作 | 说明 |
|---|---|
| 状态圆点 | 🟢 running 运行中 / 🟡 starting 启动中（进程在、端口未通）/ ⚫ stopped 已停止 / 🔴 failed 异常退出 |
| 健康 | `UP` 绿 / `DOWN` 红 / `unknown` 黄（未配置 healthUrl 或 Actuator 不可用时） |
| **日志** | 打开日志抽屉：先加载最近 300 行历史，之后实时追加；支持清屏 |
| **指标** | 打开资源弹窗：CPU% + 内存 MB 双轴曲线，保留最近 60 个采集点 |
| 启动/停止整组 | 每个分组头部按钮；顶部「全部启动/停止」= 对所有分组批量执行 |

### 状态判定逻辑

```
进程存活 && 端口可 TCP 连接 → running
进程存活 && 端口未通(刚启动) → starting
进程不在                     → stopped
```

端口为 0 的服务跳过端口探测，进程存活即视为 running。

## HTTP API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/services` | 服务列表（含运行时状态） |
| GET | `/api/services/:id` | 单个服务详情 |
| POST | `/api/services/:id/start` | 启动 |
| POST | `/api/services/:id/stop` | 停止 |
| POST | `/api/services/:id/restart` | 重启 |
| POST | `/api/groups/:group/start` | 分组批量启动 |
| POST | `/api/groups/:group/stop` | 分组批量停止 |
| GET | `/api/discovery/scan` | 立即扫描一次 |
| GET | `/api/logs/:id?lines=200` | 拉取最近 N 行历史日志 |
| WS | `/ws` | 实时日志 + 指标推送（协议见源码 `internal/ws`） |

示例：

```powershell
Invoke-RestMethod http://localhost:9090/api/services
Invoke-RestMethod -Method Post http://localhost:9090/api/services/gateway@base/start
```

## 目录结构

```
microservice-manager/
├── main.go                    # 入口：配置 → SQLite → 扫描 → manager/monitor → HTTP
├── config.yaml                # 全局配置
├── services.yaml              # 手动服务定义（优先）
├── internal/
│   ├── config/config.go       # config.yaml 解析
│   ├── store/store.go         # SQLite 持久化
│   ├── discovery/scanner.go   # jar 扫描 + application.yml 解析
│   ├── manager/manager.go     # 进程管理 / 守护 / 日志
│   ├── monitor/monitor.go     # gopsutil 采集 + Actuator 健康检查
│   ├── ws/hub.go              # WebSocket Hub
│   ├── handler/handler.go     # REST API
│   └── webdist/               # vite build 产物（embed）
└── web/                       # Vue3 + xterm.js + ECharts 前端源码
```

## 常见问题

**Q: 服务停止后进程还在？**
面板用 `taskkill /T /F` 杀整个进程树，一般不会残留；若 jar 又 fork 了脱离树的进程则无法覆盖。

**Q: 健康一直显示 unknown？**
未配置 `healthUrl`（自动发现的服务会默认填 `/actuator/health`），或服务没引 actuator 依赖。可只在 `services.yaml` 中指定其它健康地址，或无视该列。

**Q: 修改了 jar 里的配置端口怎么办？**
重新「扫描」或重启面板即可，自动发现会按 path 合并新端口（仅对未被手动定义过的服务生效）。

**Q: 数据库文件在哪？**
运行目录下的 `manager.db`，删除后下次启动会重新扫描生成。
