package monitor

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"microservice-manager/internal/config"
	"microservice-manager/internal/manager"
)

type Monitor struct {
	cfg *config.Config
	mgr *manager.Manager
	client *http.Client
	failCount map[string]int
}

func New(cfg *config.Config, mgr *manager.Manager) *Monitor {
	return &Monitor{
		cfg:    cfg,
		mgr:    mgr,
		client: &http.Client{Timeout: 3 * time.Second},
		failCount: make(map[string]int),
	}
}

func (mo *Monitor) Run() {
	go mo.metricsLoop()
	go mo.healthLoop()
}

func (mo *Monitor) metricsLoop() {
	ticker := time.NewTicker(mo.cfg.MetricsInterval)
	defer ticker.Stop()
	for range ticker.C {
		for _, id := range mo.mgr.IDs() {
			mo.collectMetrics(id)
		}
	}
}

func (mo *Monitor) collectMetrics(id string) {
	st := mo.mgr.StatusOf(id)
	if st.State != "running" && st.State != "starting" {
		mo.mgr.UpdateMetrics(id, 0, 0, 0, false)
		return
	}
	proc, err := process.NewProcess(int32(st.PID))
	if err != nil {
		mo.mgr.UpdateMetrics(id, 0, 0, 0, false)
		return
	}
	cpu, err := proc.CPUPercent()
	if err != nil {
		cpu = 0
	}
	memMB := 0.0
	if mi, err := proc.MemoryInfo(); err == nil && mi != nil {
		memMB = float64(mi.RSS) / 1024 / 1024
	}
	var threads int32
	if nt, err := proc.NumThreads(); err == nil {
		threads = nt
	}
	mo.mgr.UpdateMetrics(id, cpu, memMB, threads, true)
}

func (mo *Monitor) healthLoop() {
	ticker := time.NewTicker(mo.cfg.HealthInterval)
	defer ticker.Stop()
	for range ticker.C {
		for _, id := range mo.mgr.IDs() {
			mo.checkHealth(id)
		}
	}
}

func (mo *Monitor) checkHealth(id string) {
	st := mo.mgr.StatusOf(id)
	if st.State == "stopped" {
		mo.mgr.UpdateHealth(id, "unknown")
		return
	}
	svc := mo.serviceHealthURL(id)
	if svc == "" {
		mo.mgr.UpdateHealth(id, "unknown")
		return
	}
	resp, err := mo.client.Get(svc)
	if err != nil {
		mo.markFail(id)
		return
	}
	defer resp.Body.Close()
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Status == "" {
		mo.markFail(id)
		return
	}
	switch body.Status {
	case "UP":
		mo.failCount[id] = 0
		mo.mgr.UpdateHealth(id, "UP")
	case "DOWN":
		mo.failCount[id] = 0
		mo.mgr.UpdateHealth(id, "DOWN")
	default:
		mo.markFail(id)
	}
}

func (mo *Monitor) markFail(id string) {
	mo.failCount[id]++
	if mo.failCount[id] >= 3 {
		mo.mgr.UpdateHealth(id, "DOWN")
	} else {
		mo.mgr.UpdateHealth(id, "unknown")
	}
}

func (mo *Monitor) serviceHealthURL(id string) string {
	// avoid import cycle: manager exposes defs via StatusList flatten
	for _, item := range mo.mgr.StatusList() {
		if item["id"] == id {
			u, _ := item["healthUrl"].(string)
			return u
		}
	}
	return ""
}

var _ = log.Println
