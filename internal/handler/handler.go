package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/process"

	"microservice-manager/internal/builder"
	"microservice-manager/internal/discovery"
	"microservice-manager/internal/manager"
	"microservice-manager/internal/store"
	"microservice-manager/internal/ws"
)

type Handler struct {
	Hub     *ws.Hub
	Manager *manager.Manager
	Store   *store.Store
	Builder *builder.Builder
	ScanDir string
}

func (h *Handler) Register(mux *http.ServeMux, wsPath string) {
	mux.HandleFunc("GET /api/services", h.listServices)
	mux.HandleFunc("GET /api/services/{id}", h.getService)
	mux.HandleFunc("POST /api/services/{id}/start", h.start)
	mux.HandleFunc("POST /api/services/{id}/stop", h.stop)
	mux.HandleFunc("POST /api/services/{id}/restart", h.restart)
	mux.HandleFunc("DELETE /api/services/{id}", h.remove)
	mux.HandleFunc("PUT /api/services/{id}", h.updateService)
	mux.HandleFunc("POST /api/services", h.createService)
	mux.HandleFunc("POST /api/services/{id}/open-dir", h.openDir)
	mux.HandleFunc("POST /api/groups/{group}/start", h.groupStart)
	mux.HandleFunc("POST /api/groups/{group}/stop", h.groupStop)
	mux.HandleFunc("GET /api/discovery/scan", h.scan)
	mux.HandleFunc("GET /api/logs/{id}", h.logs)
	mux.HandleFunc("GET /api/settings", h.getSettings)
	mux.HandleFunc("POST /api/settings", h.setSettings)
	mux.HandleFunc("POST /api/build", h.buildStart)
	mux.HandleFunc("POST /api/build/stop", h.buildStop)
	mux.HandleFunc("GET /api/build", h.buildStatus)
	mux.HandleFunc("GET /api/system/processes", h.sysProcesses)
	mux.HandleFunc("POST /api/system/kill", h.sysKill)
	mux.HandleFunc("POST /api/system/open-dir", h.sysOpenDir)
	mux.HandleFunc(wsPath, h.Hub.ServeWS)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.Manager.StatusList())
}

func (h *Handler) getService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, item := range h.Manager.StatusList() {
		if item["id"] == id {
			writeJSON(w, 200, item)
			return
		}
	}
	writeJSON(w, 404, map[string]string{"error": "not found"})
}

func (h *Handler) ok(w http.ResponseWriter, r *http.Request, fn func(string) error, idKey string) {
	id := r.PathValue(idKey)
	if err := fn(id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Manager.Remove(id); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := h.Store.Delete(id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) updateService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sv, err := h.Store.GetByID(id)
	if err != nil || sv == nil {
		writeJSON(w, 404, map[string]string{"error": "service not found"})
		return
	}
	var body struct {
		Port        int     `json:"port"`
		Group       *string `json:"group"`
		Description *string `json:"description"`
		JavaOpts    *string `json:"javaOpts"`
		Command     *string `json:"command"`
		WorkDir     *string `json:"workDir"`
		HealthURL   *string `json:"healthUrl"`
		AutoRestart *bool   `json:"autoRestart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	sv.Port = body.Port
	if body.Group != nil {
		sv.Group = *body.Group
	}
	if body.Description != nil {
		sv.Description = *body.Description
	}
	if body.JavaOpts != nil {
		sv.JavaOpts = *body.JavaOpts
	}
	if body.Command != nil {
		sv.Command = *body.Command
	}
	if body.WorkDir != nil {
		sv.WorkDir = *body.WorkDir
	}
	if body.AutoRestart != nil {
		sv.AutoRestart = *body.AutoRestart
	}
	if body.HealthURL != nil {
		sv.HealthURL = *body.HealthURL
	} else if body.Port > 0 && sv.HealthURL == "" {
		sv.HealthURL = fmt.Sprintf("http://localhost:%d/actuator/health", body.Port)
	}
	if err := h.Store.Update(sv); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.Manager.SetService(sv)
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) createService(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Group       string `json:"group"`
		Command     string `json:"command"`
		WorkDir     string `json:"workDir"`
		Port        int    `json:"port"`
		Description string `json:"description"`
		HealthURL   string `json:"healthUrl"`
		AutoRestart *bool  `json:"autoRestart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Command) == "" {
		writeJSON(w, 400, map[string]string{"error": "name 与 command 必填"})
		return
	}
	group := body.Group
	if group == "" {
		group = "default"
	}
	sv := &store.Service{
		ID:          body.Name + "@" + group,
		Name:        body.Name,
		Group:       group,
		Type:        "custom",
		Path:        "custom:" + body.Name + "@" + group,
		WorkDir:     body.WorkDir,
		Command:     body.Command,
		Port:        body.Port,
		Description: body.Description,
		HealthURL:   body.HealthURL,
		AutoRestart: true,
		Enabled:     true,
	}
	if body.AutoRestart != nil {
		sv.AutoRestart = *body.AutoRestart
	}
	if err := h.Store.Upsert(sv); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.Manager.SetService(sv)
	writeJSON(w, 200, sv)
}

func (h *Handler) openDir(w http.ResponseWriter, r *http.Request) {
	sv, err := h.Store.GetByID(r.PathValue("id"))
	if err != nil || sv == nil {
		writeJSON(w, 404, map[string]string{"error": "service not found"})
		return
	}
	target := sv.WorkDir
	if target == "" {
		target = filepath.Dir(sv.Path)
	}
	if target == "" || target == "." {
		writeJSON(w, 400, map[string]string{"error": "没有可打开的目录"})
		return
	}
	// explorer 定位到 jar 文件；目录则直接打开
	if strings.HasSuffix(strings.ToLower(sv.Path), ".jar") {
		exec.Command("explorer", "/select,"+sv.Path).Start()
	} else {
		exec.Command("explorer", target).Start()
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) sysProcesses(w http.ResponseWriter, r *http.Request) {
	procs, err := process.Processes()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	self := int32(os.Getpid())
	type procInfo struct {
		PID   int32   `json:"pid"`
		Name  string  `json:"name"`
		CPU   float64 `json:"cpu"`
		MemMB float64 `json:"memMb"`
		Exe   string  `json:"exe"`
		Self  bool    `json:"self"`
	}
	out := make([]procInfo, 0, len(procs))
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		cpu, _ := p.CPUPercent()
		memMB := 0.0
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			memMB = float64(mi.RSS) / 1024 / 1024
		}
		exe, _ := p.Exe()
		pid := p.Pid
		out = append(out, procInfo{PID: pid, Name: name, CPU: cpu, MemMB: memMB, Exe: exe, Self: pid == self})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MemMB > out[j].MemMB })
	writeJSON(w, 200, out)
}

func (h *Handler) sysKill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PID int32 `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PID <= 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid pid"})
		return
	}
	if body.PID <= 4 || body.PID == int32(os.Getpid()) {
		writeJSON(w, 400, map[string]string{"error": "拒绝结束系统关键进程或面板自身"})
		return
	}
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(int(body.PID))).Run(); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) sysOpenDir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid path"})
		return
	}
	exec.Command("explorer", "/select,"+body.Path).Start()
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	h.ok(w, r, func(id string) error { return h.Manager.Start(id) }, "id")
}
func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	h.ok(w, r, func(id string) error { return h.Manager.Stop(id) }, "id")
}
func (h *Handler) restart(w http.ResponseWriter, r *http.Request) {
	h.ok(w, r, func(id string) error { return h.Manager.Restart(id) }, "id")
}
func (h *Handler) groupStart(w http.ResponseWriter, r *http.Request) {
	h.Manager.StartGroup(r.PathValue("group"))
	writeJSON(w, 200, map[string]string{"ok": "true"})
}
func (h *Handler) groupStop(w http.ResponseWriter, r *http.Request) {
	h.Manager.StopGroup(r.PathValue("group"))
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) scan(w http.ResponseWriter, r *http.Request) {
	// optional ?dir= 指定扫描目录并持久化
	if dir := strings.TrimSpace(r.URL.Query().Get("dir")); dir != "" {
		h.Store.SetSetting("scanDir", dir)
		h.ScanDir = dir
	}
	res, err := discovery.Scan(h.ScanDir, h.Store)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	for _, sv := range res.New {
		h.Manager.SetService(sv)
	}
	writeJSON(w, 200, map[string]interface{}{
		"scanDir":  h.ScanDir,
		"new":      len(res.New),
		"existing": res.Existing,
		"services": res.New,
	})
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"scanDir": h.ScanDir})
}

func (h *Handler) setSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScanDir string `json:"scanDir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	h.ScanDir = strings.TrimSpace(body.ScanDir)
	h.Store.SetSetting("scanDir", h.ScanDir)
	// 立即按新目录扫描
	res, err := discovery.Scan(h.ScanDir, h.Store)
	if err == nil {
		for _, sv := range res.New {
			h.Manager.SetService(sv)
		}
	}
	writeJSON(w, 200, map[string]interface{}{"ok": "true", "scanDir": h.ScanDir})
}

func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if lines <= 0 {
		lines = 200
	}
	if id == builder.BuildServiceID {
		writeJSON(w, 200, map[string]interface{}{"serviceId": id, "lines": h.Builder.Tail(lines)})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"serviceId": id,
		"lines":     h.Manager.Tail(id, lines),
	})
}

func (h *Handler) buildStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.Builder.Start(strings.TrimSpace(body.Dir)); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) buildStop(w http.ResponseWriter, r *http.Request) {
	if err := h.Builder.Stop(); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *Handler) buildStatus(w http.ResponseWriter, r *http.Request) {
	running, dir, lastExit := h.Builder.Status()
	writeJSON(w, 200, map[string]interface{}{
		"running":  running,
		"dir":      dir,
		"lastExit": lastExit,
	})
}

var _ = websocket.TextMessage
var _ = strings.TrimSpace
