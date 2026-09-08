package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"microservice-manager/internal/config"
	"microservice-manager/internal/discovery"
	"microservice-manager/internal/handler"
	"microservice-manager/internal/manager"
	"microservice-manager/internal/monitor"
	"microservice-manager/internal/store"
	"microservice-manager/internal/ws"

	"gopkg.in/yaml.v3"
)

//go:embed all:internal/webdist
var webdist embed.FS

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	os.MkdirAll(cfg.LogDir, 0755)

	st, err := store.Open("manager.db")
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()

	// load services.yaml (manual definitions take priority on conflict)
	loadServicesYaml(cfg, st)

	// initial discovery scan
	if res, err := discovery.Scan(cfg.ScanDir, st); err == nil {
		log.Printf("discovery: %d new, %d existing", len(res.New), res.Existing)
	}

	svcs, err := st.GetAll()
	if err != nil {
		log.Fatalf("load services: %v", err)
	}

	hub := ws.NewHub()
	mgr := manager.New(cfg, hub, st)
	for _, sv := range svcs {
		mgr.SetService(sv)
	}

	mon := monitor.New(cfg, mgr)
	mon.Run()

	// periodic rediscovery
	go func() {
		ticker := time.NewTicker(cfg.ScanInterval)
		defer ticker.Stop()
		for range ticker.C {
			if res, err := discovery.Scan(cfg.ScanDir, st); err == nil {
				for _, sv := range res.New {
					mgr.SetService(sv)
				}
			}
		}
	}()

	h := &handler.Handler{Hub: hub, Manager: mgr, Store: st, ScanDir: cfg.ScanDir}
	mux := http.NewServeMux()
	h.Register(mux, cfg.WsPath)

	sub, err := fs.Sub(webdist, "internal/webdist")
	if err != nil {
		log.Fatalf("embed webdist: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Listening on http://localhost:%d", cfg.Server.Port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func loadServicesYaml(cfg *config.Config, st *store.Store) {
	type svcDef struct {
		Name        string `yaml:"name"`
		Type        string `yaml:"type"`
		Path        string `yaml:"path"`
		JavaOpts    string `yaml:"javaOpts"`
		Port        int    `yaml:"port"`
		HealthURL   string `yaml:"healthUrl"`
		Group       string `yaml:"group"`
		AutoRestart *bool  `yaml:"autoRestart"`
	}
	var doc struct {
		ScanDir  string   `yaml:"scanDir"`
		Services []svcDef `yaml:"services"`
	}
	data, err := os.ReadFile("services.yaml")
	if err != nil {
		return
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		log.Printf("parse services.yaml: %v", err)
		return
	}
	if doc.ScanDir != "" && cfg.ScanDir == "" {
		cfg.ScanDir = doc.ScanDir
	}
	for _, d := range doc.Services {
		if d.Path == "" {
			continue
		}
		sv := &store.Service{
			ID:          d.Name + "@" + orDefault(d.Group, "default"),
			Name:        d.Name,
			Group:       orDefault(d.Group, "default"),
			Type:        orDefault(d.Type, "jar"),
			Path:        d.Path,
			JavaOpts:    d.JavaOpts,
			Port:        d.Port,
			HealthURL:   d.HealthURL,
			AutoRestart: true,
			Enabled:     true,
		}
		if d.AutoRestart != nil {
			sv.AutoRestart = *d.AutoRestart
		}
		if err := st.Upsert(sv); err != nil {
			log.Printf("upsert %s: %v", d.Name, err)
		}
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
