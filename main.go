package main

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
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
		fatalf("load config: %v", err)
	}
	os.MkdirAll(cfg.LogDir, 0755)

	st, err := store.Open("manager.db")
	if err != nil {
		fatalf("open sqlite: %v", err)
	}
	defer st.Close()

	// load services.yaml (manual definitions take priority on conflict)
	loadServicesYaml(cfg, st)

	// UI 中设置的扫描目录优先于 config.yaml
	if dir := st.GetSetting("scanDir"); dir != "" {
		cfg.ScanDir = dir
	}

	// initial discovery scan
	if res, err := discovery.Scan(cfg.ScanDir, st); err == nil {
		log.Printf("discovery: %d new, %d existing", len(res.New), res.Existing)
	}

	svcs, err := st.GetAll()
	if err != nil {
		fatalf("load services: %v", err)
	}

	hub := ws.NewHub()
	mgr := manager.New(cfg, hub, st)
	for _, sv := range svcs {
		mgr.SetService(sv)
	}

	mon := monitor.New(cfg, mgr)
	mon.Run()

	// periodic rediscovery (read latest scanDir each tick, UI may change it)
	go func() {
		ticker := time.NewTicker(cfg.ScanInterval)
		defer ticker.Stop()
		for range ticker.C {
			dir := st.GetSetting("scanDir")
			if dir == "" {
				dir = cfg.ScanDir
			}
			if res, err := discovery.Scan(dir, st); err == nil {
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
		fatalf("embed webdist: %v", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// index.html 不缓存，避免升级后浏览器引用旧的 hash 资源导致白屏
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Listening on http://localhost:%d", cfg.Server.Port)

	// 记录每个请求的耗时与状态（诊断用）
	logFile, _ := os.OpenFile("http-debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	logged := http.NewServeMux()
	logged.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// WebSocket 升级需要原生 ResponseWriter（Hijacker），不能包装
		if r.URL.Path == cfg.WsPath {
			mux.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, code: 200}
		start := time.Now()
		mux.ServeHTTP(sw, r)
		line := fmt.Sprintf("%s %s -> %d (%dms)\n", r.Method, r.URL.Path, sw.code, time.Since(start).Milliseconds())
		logFile.WriteString(line)
		fmt.Print(line)
	})

	if err := http.ListenAndServe(addr, logged); err != nil {
		fmt.Printf("\n[启动失败] %v\n", err)
		if strings.Contains(err.Error(), "Only one usage") || strings.Contains(err.Error(), "being used") {
			fmt.Printf("端口 %d 已被占用：可能面板已在运行（浏览器打开 http://localhost:%d 试试），\n或先结束旧进程：taskkill /F /IM micro-manager.exe\n", cfg.Server.Port, cfg.Server.Port)
		}
		fmt.Println("\n按回车键关闭窗口...")
		bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(1)
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Printf("[启动失败] "+format+"\n按回车键关闭窗口...\n", args...)
	bufio.NewReader(os.Stdin).ReadString('\n')
	os.Exit(1)
}

type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (s *statusWriter) WriteHeader(code int) {
	if !s.written {
		s.code = code
		s.written = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	s.written = true
	return s.ResponseWriter.Write(b)
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
