package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"microservice-manager/internal/discovery"
	"microservice-manager/internal/manager"
	"microservice-manager/internal/store"
	"microservice-manager/internal/ws"
)

type Handler struct {
	Hub     *ws.Hub
	Manager *manager.Manager
	Store   *store.Store
	ScanDir string
}

func (h *Handler) Register(mux *http.ServeMux, wsPath string) {
	mux.HandleFunc("GET /api/services", h.listServices)
	mux.HandleFunc("GET /api/services/{id}", h.getService)
	mux.HandleFunc("POST /api/services/{id}/start", h.start)
	mux.HandleFunc("POST /api/services/{id}/stop", h.stop)
	mux.HandleFunc("POST /api/services/{id}/restart", h.restart)
	mux.HandleFunc("POST /api/groups/{group}/start", h.groupStart)
	mux.HandleFunc("POST /api/groups/{group}/stop", h.groupStop)
	mux.HandleFunc("GET /api/discovery/scan", h.scan)
	mux.HandleFunc("GET /api/logs/{id}", h.logs)
	mux.HandleFunc("GET /api/settings", h.getSettings)
	mux.HandleFunc("POST /api/settings", h.setSettings)
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
	writeJSON(w, 200, map[string]interface{}{
		"serviceId": id,
		"lines":     h.Manager.Tail(id, lines),
	})
}

var _ = websocket.TextMessage
var _ = strings.TrimSpace
