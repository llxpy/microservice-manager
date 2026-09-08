package manager

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"microservice-manager/internal/config"
	"microservice-manager/internal/store"
	"microservice-manager/internal/ws"
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
	defer m.mu.Unlock()
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
}

func (m *Manager) get(id string) *store.Service {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.services[id]
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
		m.hub.Broadcast(map[string]interface{}{"type": "list", "services": m.StatusList()})
	}
}

func (m *Manager) Start(id string) error {
	sv := m.get(id)
	if sv == nil {
		return errors.New("service not found: " + id)
	}
	m.mu.Lock()
	if _, ok := m.procs[id]; ok {
		m.mu.Unlock()
		return errors.New("service already running")
	}
	m.mu.Unlock()

	args := append([]string{}, strings.Fields(sv.JavaOpts)...)
	args = append(args, "-jar", sv.Path)
	workDir := sv.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(sv.Path)
	}
	cmd := exec.Command("java", args...)
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
	if sv.AutoRestart {
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
	kill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", entry.pid))
	kill.Run()

	select {
	case <-entry.waitDone:
	case <-time.After(m.cfg.StopTimeout):
		exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", entry.pid)).Run()
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

func (m *Manager) UpdateHealth(id, health string) {
	m.mu.Lock()
	s := m.statuses[id]
	if s != nil {
		s.Health = health
	}
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
		"port": sv.Port, "healthUrl": sv.HealthURL,
		"autoRestart": sv.AutoRestart, "enabled": sv.Enabled,
		"state": st.State, "pid": st.PID, "health": st.Health,
		"cpu": st.CPU, "memMb": st.MemMB, "threads": st.Threads, "uptime": st.Uptime,
	}
}
