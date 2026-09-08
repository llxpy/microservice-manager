package manager

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"microservice-manager/internal/config"
	"microservice-manager/internal/executil"
	"microservice-manager/internal/store"
	"microservice-manager/internal/ws"

	"github.com/shirou/gopsutil/v3/process"
)

type Status struct {
	ServiceID string  `json:"serviceId"`
	State     string  `json:"state"` // running | stopped | starting | failed
	PID       int     `json:"pid"`
	Port      int     `json:"port"`
	Health    string  `json:"health"` // UP | DOWN | unknown
	CPU       float64 `json:"cpu"`
	MemMB     float64 `json:"memMb"`
	Threads   int32   `json:"threads"`
	Uptime    int64   `json:"uptime"` // start unix timestamp
}

type procEntry struct {
	cmd       *exec.Cmd
	pid       int
	startedAt time.Time
	manual    atomic.Bool
	waitDone  chan struct{}
}

type ringBuf struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func (r *ringBuf) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, s)
	if len(r.lines) > r.max {
		r.lines = r.lines[len(r.lines)-r.max:]
	}
}

func (r *ringBuf) tail(n int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || n > len(r.lines) {
		n = len(r.lines)
	}
	out := make([]string, n)
	copy(out, r.lines[len(r.lines)-n:])
	return out
}

type logFile struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	f        *os.File
	size     int64
}

func (w *logFile) write(line string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		if st, err := os.Stat(w.path); err == nil && st.Size() >= w.maxBytes {
			w.rotateLocked()
		}
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		w.f = f
		if st, err := f.Stat(); err == nil {
			w.size = st.Size()
		}
	}
	n, _ := w.f.WriteString(line)
	w.size += int64(n)
	if w.size >= w.maxBytes {
		w.rotateLocked()
	}
}

func (w *logFile) rotateLocked() {
	if w.f != nil {
		w.f.Close()
		w.f = nil
	}
	for i := 2; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
	}
	os.Rename(w.path, w.path+".1")
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err == nil {
		w.f = f
		w.size = 0
	}
}

type Manager struct {
	cfg      *config.Config
	hub      *ws.Hub
	st       *store.Store
	mu       sync.RWMutex
	services map[string]*store.Service
	procs    map[string]*procEntry
	statuses map[string]*Status
	buffers  map[string]*ringBuf
	loggers  map[string]*logFile
}

func New(cfg *config.Config, hub *ws.Hub, st *store.Store) *Manager {
	return &Manager{
		cfg:      cfg,
		hub:      hub,
		st:       st,
		services: make(map[string]*store.Service),
		procs:    make(map[string]*procEntry),
		statuses: make(map[string]*Status),
		buffers:  make(map[string]*ringBuf),
		loggers:  make(map[string]*logFile),
	}
}

func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
}

func (m *Manager) SetService(sv *store.Service) {
	m.mu.Lock()
	_, existed := m.services[sv.ID]
	m.services[sv.ID] = sv
	if _, ok := m.buffers[sv.ID]; !ok {
		m.buffers[sv.ID] = &ringBuf{max: 1000}
	}
	if _, ok := m.loggers[sv.ID]; !ok {
		m.loggers[sv.ID] = &logFile{
			path:     filepath.Join(m.cfg.LogDir, sanitize(sv.Name)+".log"),
			maxBytes: m.cfg.LogMaxSizeMB * 1024 * 1024,
		}
	}
	if _, ok := m.statuses[sv.ID]; !ok {
		m.statuses[sv.ID] = &Status{ServiceID: sv.ID, State: "stopped", Health: "unknown"}
	}
	m.mu.Unlock()
	// 仅在新增服务时广播列表，避免每次状态更新都全量刷前端
	if !existed {
		m.hub.Broadcast(map[string]interface{}{"type": "list", "services": m.StatusList()})
	}
}

func (m *Manager) get(id string) *store.Service {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.services[id]
}

// Remove 移除纳管（运行中的服务先停止）。返回错误表示服务正在运行需先停止。
func (m *Manager) Remove(id string) error {
	if m.Alive(id) {
		return errors.New("服务运行中，请先停止")
	}
	m.mu.Lock()
	delete(m.services, id)
	delete(m.procs, id)
	delete(m.statuses, id)
	delete(m.buffers, id)
	lg := m.loggers[id]
	delete(m.loggers, id)
	m.mu.Unlock()
	if lg != nil {
		lg.mu.Lock()
		if lg.f != nil {
			lg.f.Close()
			lg.f = nil
		}
		lg.mu.Unlock()
	}
	m.hub.Broadcast(map[string]interface{}{"type": "list", "services": m.StatusList()})
	return nil
}

func (m *Manager) IDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.services))
	for id := range m.services {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) Alive(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.procs[id]
	return ok
}

func (m *Manager) writePidFile(id, name string, pid int) {
	path := filepath.Join(m.cfg.LogDir, sanitize(name)+".pid")
	if pid <= 0 {
		os.Remove(path)
		return
	}
	os.WriteFile(path, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func (m *Manager) setState(id, state string) {
	m.mu.Lock()
	s := m.statuses[id]
	if s == nil {
		s = &Status{ServiceID: id, Health: "unknown"}
		m.statuses[id] = s
	}
	s.State = state
	if state == "stopped" || state == "failed" {
		s.PID = 0
		s.Uptime = 0
		s.CPU = 0
		s.MemMB = 0
		s.Threads = 0
	}
	m.mu.Unlock()
	m.broadcastStatus(id)
}

func (m *Manager) broadcastStatus(id string) {
	m.mu.RLock()
	s := m.statuses[id]
	m.mu.RUnlock()
	if s != nil {
		m.hub.Broadcast(map[string]interface{}{"type": "status", "serviceId": id, "status": s})
	}
}

var reTomcatPort = regexp.MustCompile(`Tomcat started on port(?:\(s\))?:?\s+(\d+)`)

// detectPort 从启动日志中识别实际监听端口（配置在 Nacos 等配置中心时本地 yml 解析不到）
func (m *Manager) detectPort(id, line string) {
	match := reTomcatPort.FindStringSubmatch(line)
	if match == nil {
		return
	}
	port, _ := strconv.Atoi(match[1])
	if port <= 0 || port > 65535 {
		return
	}
	m.mu.RLock()
	sv := m.services[id]
	s := m.statuses[id]
	m.mu.RUnlock()
	if sv == nil || sv.Port == port {
		return
	}
	sv.Port = port
	if sv.HealthURL == "" {
		sv.HealthURL = fmt.Sprintf("http://localhost:%d/actuator/health", port)
	}
	m.st.Update(sv)
	m.mu.Lock()
	if s != nil {
		s.Port = port
	}
	m.mu.Unlock()
	m.emitLog(id, fmt.Sprintf("[manager] 已从日志识别服务端口: %d\n", port))
	m.broadcastStatus(id)
}

// SyncState 由监控循环定期调用，状态发生变化时推送给前端
func (m *Manager) SyncState(id, state string) {
	m.mu.Lock()
	s := m.statuses[id]
	if s == nil || s.State == state {
		m.mu.Unlock()
		return
	}
	s.State = state
	m.mu.Unlock()
	m.broadcastStatus(id)
}

func (m *Manager) Start(id string) error {
	sv := m.get(id)
	if sv == nil {
		return errors.New("service not found: " + id)
	}
	m.mu.Lock()
	if _, ok := m.procs[id]; ok {
		m.mu.Unlock()
		return errors.New("服务已在运行中")
	}
	m.mu.Unlock()

	// 启动前预检端口占用，直接给出占用者信息，避免启动后绑定失败
	if sv.Port > 0 {
		if pid, pname := portOwner(sv.Port); pid > 0 {
			return fmt.Errorf("端口 %d 已被 PID %d (%s) 占用：请先停止该进程，或在「编辑」中修改服务端口", sv.Port, pid, pname)
		}
	}

	var cmd *exec.Cmd
	if sv.Type == "jar" {
		args := append([]string{}, strings.Fields(sv.JavaOpts)...)
		args = append(args, "-jar", sv.Path)
		cmd = executil.Command("java", args...)
	} else {
		// 任意语言：直接执行启动命令（cmd /c python app.py / node server.js ...）
		if sv.Command == "" {
			return errors.New("服务缺少启动命令")
		}
		cmd = executil.Command("cmd", "/c", sv.Command)
	}
	workDir := sv.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(sv.Path)
	}
	cmd.Dir = workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		m.setState(id, "failed")
		return fmt.Errorf("start java failed: %w", err)
	}

	entry := &procEntry{
		cmd:       cmd,
		pid:       cmd.Process.Pid,
		startedAt: time.Now(),
		waitDone:  make(chan struct{}),
	}
	m.mu.Lock()
	m.procs[id] = entry
	if s := m.statuses[id]; s != nil {
		s.PID = entry.pid
		s.Uptime = entry.startedAt.Unix()
		s.Port = sv.Port
	}
	m.mu.Unlock()
	m.writePidFile(id, sv.Name, entry.pid)
	m.setState(id, "starting")

	go m.pipe(id, stdout)
	go m.pipe(id, stderr)
	go m.waitLoop(id, sv, entry)
	return nil
}

func (m *Manager) pipe(id string, r interface{ Read([]byte) (int, error) }) {
	buf := make([]byte, 4096)
	var pending []byte
	for {
		n, err := r.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			for {
				idx := -1
				for i, b := range pending {
					if b == '\n' {
						idx = i
						break
					}
				}
				if idx < 0 {
					break
				}
				line := string(pending[:idx+1])
				pending = pending[idx+1:]
				m.emitLog(id, line)
			}
			if len(pending) > 65536 {
				m.emitLog(id, string(pending)+"\n")
				pending = nil
			}
		}
		if err != nil {
			if len(pending) > 0 {
				m.emitLog(id, string(pending)+"\n")
			}
			return
		}
	}
}

func (m *Manager) emitLog(id, line string) {
	m.detectPort(id, line)
	m.mu.RLock()
	buf := m.buffers[id]
	lg := m.loggers[id]
	m.mu.RUnlock()
	if buf != nil {
		buf.add(line)
	}
	if lg != nil {
		lg.write(line)
	}
	m.hub.BroadcastTo(id, map[string]interface{}{"type": "log", "serviceId": id, "data": line})
}

func (m *Manager) waitLoop(id string, sv *store.Service, entry *procEntry) {
	err := entry.cmd.Wait()
	close(entry.waitDone)

	m.mu.Lock()
	cur, ok := m.procs[id]
	if ok && cur == entry {
		delete(m.procs, id)
	}
	m.mu.Unlock()
	m.writePidFile(id, sv.Name, 0)

	wasManual := entry.manual.Load()
	if wasManual {
		m.setState(id, "stopped")
		return
	}
	if err != nil {
		m.emitLog(id, fmt.Sprintf("[manager] process exited: %v\n", err))
	} else {
		m.emitLog(id, "[manager] process exited\n")
	}
	// 防崩溃循环：只有运行满 1 分钟的进程才允许自动重启
	if sv.AutoRestart && time.Since(entry.startedAt) >= time.Minute {
		m.setState(id, "starting")
		m.emitLog(id, fmt.Sprintf("[manager] auto restarting in %s\n", m.cfg.RestartDelay))
		time.Sleep(m.cfg.RestartDelay)
		// only restart if nobody manually started/stopped meanwhile
		if !m.Alive(id) {
			if serr := m.Start(id); serr != nil {
				m.setState(id, "failed")
			}
		}
	} else {
		if sv.AutoRestart {
			m.emitLog(id, "[manager] 进程运行不足 1 分钟即退出，禁止自动重启（防崩溃循环）\n")
		}
		m.setState(id, "failed")
	}
}

func (m *Manager) Stop(id string) error {
	m.mu.RLock()
	entry := m.procs[id]
	sv := m.services[id]
	m.mu.RUnlock()
	if entry == nil {
		return nil // already stopped
	}
	entry.manual.Store(true)

	// Windows: kill whole process tree
	kill := executil.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", entry.pid))
	kill.Run()

	select {
	case <-entry.waitDone:
	case <-time.After(m.cfg.StopTimeout):
		executil.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", entry.pid)).Run()
		select {
		case <-entry.waitDone:
		case <-time.After(3 * time.Second):
		}
	}
	_ = sv
	m.setState(id, "stopped")
	return nil
}

func (m *Manager) Restart(id string) error {
	_ = m.Stop(id)
	time.Sleep(500 * time.Millisecond)
	return m.Start(id)
}

func (m *Manager) GroupServices(group string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for id, sv := range m.services {
		if sv.Group == group {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m *Manager) startGroupAsync(ids []string, action string) {
	for _, id := range ids {
		id := id
		go func() {
			var err error
			switch action {
			case "start":
				err = m.Start(id)
			case "stop":
				err = m.Stop(id)
			case "restart":
				err = m.Restart(id)
			}
			_ = err
		}()
	}
}

func (m *Manager) StartGroup(group string) { m.startGroupAsync(m.GroupServices(group), "start") }
func (m *Manager) StopGroup(group string)  { m.startGroupAsync(m.GroupServices(group), "stop") }

// GroupProjectRoot 定位分组对应的 Maven 项目根目录（从组内 jar 向上找最外层 pom.xml）
func (m *Manager) GroupProjectRoot(group string) string {
	m.mu.RLock()
	var jarDir string
	for _, sv := range m.services {
		if sv.Group == group && sv.Type == "jar" && sv.Path != "" {
			jarDir = filepath.Dir(sv.Path)
			break
		}
	}
	m.mu.RUnlock()
	if jarDir == "" {
		return ""
	}
	root := ""
	dir := jarDir
	for i := 0; i < 8; i++ {
		if fileExistsPom(dir) {
			root = dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return root
}

func fileExistsPom(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "pom.xml"))
	return err == nil && !st.IsDir()
}

// RemoveGroup 停止并移除分组下所有服务（不删除文件），返回移除数量
func (m *Manager) RemoveGroup(group string) int {
	ids := m.GroupServices(group)
	count := 0
	for _, id := range ids {
		if m.Alive(id) {
			m.Stop(id)
		}
		m.mu.Lock()
		delete(m.services, id)
		delete(m.procs, id)
		delete(m.statuses, id)
		delete(m.buffers, id)
		lg := m.loggers[id]
		delete(m.loggers, id)
		m.mu.Unlock()
		if lg != nil {
			lg.mu.Lock()
			if lg.f != nil {
				lg.f.Close()
				lg.f = nil
			}
			lg.mu.Unlock()
		}
		m.st.Delete(id)
		count++
	}
	if count > 0 {
		m.hub.Broadcast(map[string]interface{}{"type": "list", "services": m.StatusList()})
	}
	return count
}

func (m *Manager) Tail(id string, lines int) []string {
	m.mu.RLock()
	buf := m.buffers[id]
	m.mu.RUnlock()
	if buf == nil {
		return []string{}
	}
	return buf.tail(lines)
}

// UpdateMetrics stores resource metrics collected by monitor.
func (m *Manager) UpdateMetrics(id string, cpu, memMB float64, threads int32, alive bool) {
	m.mu.Lock()
	s := m.statuses[id]
	if s == nil {
		m.mu.Unlock()
		return
	}
	if !alive {
		s.CPU, s.MemMB, s.Threads = 0, 0, 0
		m.mu.Unlock()
		return
	}
	s.CPU, s.MemMB, s.Threads = cpu, memMB, threads
	m.mu.Unlock()
}

// UpdateHealth stores health result; skips broadcast when unchanged.
func (m *Manager) UpdateHealth(id, health string) {
	m.mu.Lock()
	s := m.statuses[id]
	if s == nil {
		m.mu.Unlock()
		return
	}
	if s.Health == health {
		m.mu.Unlock()
		return
	}
	s.Health = health
	m.mu.Unlock()
	m.broadcastStatus(id)
}

func (m *Manager) StatusOf(id string) *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.statuses[id]
	if s == nil {
		return &Status{ServiceID: id, State: "stopped", Health: "unknown"}
	}
	cp := *s
	// refine state: alive + port probe
	entry, alive := m.procs[id]
	if alive {
		cp.Uptime = entry.startedAt.Unix()
		cp.PID = entry.pid
		cp.State = "running"
		sv := m.services[id]
		if sv != nil && sv.Port > 0 {
			if !portOpen(sv.Port) {
				cp.State = "starting"
			}
		}
	} else {
		cp.State = "stopped"
	}
	return &cp
}

// portOwner 查询正在监听指定端口的进程（pid + 进程名），无占用者返回 0
func portOwner(port int) (int, string) {
	out, err := executil.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return 0, ""
	}
	suffix := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// TCP  0.0.0.0:28001  0.0.0.0:0  LISTENING  22020
		if len(fields) < 5 || !strings.EqualFold(fields[len(fields)-2], "LISTENING") {
			continue
		}
		if !strings.HasSuffix(fields[1], suffix) {
			continue
		}
		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || pid <= 0 {
			continue
		}
		name := "unknown"
		if p, err := process.NewProcess(int32(pid)); err == nil {
			if n, err := p.Name(); err == nil {
				name = n
			}
		}
		return pid, name
	}
	return 0, ""
}

func portOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (m *Manager) StatusList() []map[string]interface{} {
	m.mu.RLock()
	svs := make([]*store.Service, 0, len(m.services))
	for _, sv := range m.services {
		svs = append(svs, sv)
	}
	m.mu.RUnlock()

	out := make([]map[string]interface{}, 0, len(svs))
	for _, sv := range svs {
		st := m.StatusOf(sv.ID)
		out = append(out, flatten(sv, st))
	}
	return out
}

func flatten(sv *store.Service, st *Status) map[string]interface{} {
	return map[string]interface{}{
		"id": sv.ID, "name": sv.Name, "group": sv.Group, "type": sv.Type,
		"path": sv.Path, "workDir": sv.WorkDir, "javaOpts": sv.JavaOpts,
		"command": sv.Command, "description": sv.Description,
		"port": sv.Port, "healthUrl": sv.HealthURL,
		"autoRestart": sv.AutoRestart, "enabled": sv.Enabled,
		"state": st.State, "pid": st.PID, "health": st.Health,
		"cpu": st.CPU, "memMb": st.MemMB, "threads": st.Threads, "uptime": st.Uptime,
	}
}
