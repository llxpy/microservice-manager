package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"

	"microservice-manager/internal/builder"
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

const version = "v1.0"

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fatalUI("读取配置失败", err.Error())
	}
	os.MkdirAll(cfg.LogDir, 0755)

	setupLogging(cfg)

	st, err := store.Open("manager.db")
	if err != nil {
		fatalUI("打开数据库失败", err.Error())
	}
	defer st.Close()

	loadServicesYaml(cfg, st)

	if dir := st.GetSetting("scanDir"); dir != "" {
		cfg.ScanDir = dir
	}
	if res, err := discovery.Scan(cfg.ScanDir, st); err == nil {
		log.Printf("discovery: %d new, %d existing", len(res.New), res.Existing)
	}

	svcs, err := st.GetAll()
	if err != nil {
		fatalUI("加载服务失败", err.Error())
	}

	hub := ws.NewHub()
	mgr := manager.New(cfg, hub, st)
	for _, sv := range svcs {
		mgr.SetService(sv)
	}

	bldr := builder.New(hub)
	bldr.OnDone = func(dir string, success bool) { rescanAll(cfg, st, mgr) }

	mon := monitor.New(cfg, mgr)
	mon.Run()

	go func() {
		ticker := time.NewTicker(cfg.ScanInterval)
		defer ticker.Stop()
		for range ticker.C {
			rescanAll(cfg, st, mgr)
		}
	}()

	h := &handler.Handler{Hub: hub, Manager: mgr, Store: st, Builder: bldr, ScanDir: cfg.ScanDir}
	mux := http.NewServeMux()
	h.Register(mux, cfg.WsPath)

	sub, err := fs.Sub(webdist, "internal/webdist")
	if err != nil {
		fatalUI("内部错误", err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		fileServer.ServeHTTP(w, r)
	})

	logFile, _ := os.OpenFile("http-debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	logged := http.NewServeMux()
	logged.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == cfg.WsPath {
			mux.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, code: 200}
		start := time.Now()
		mux.ServeHTTP(sw, r)
		logFile.WriteString(fmt.Sprintf("%s %s -> %d (%dms)\n", r.Method, r.URL.Path, sw.code, time.Since(start).Milliseconds()))
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Listening on %s", addr)

	go func() {
		if err := http.ListenAndServe(addr, logged); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "Only one usage") || strings.Contains(msg, "being used") {
				msg = fmt.Sprintf("端口 %d 已被占用：可能面板已在运行，或先结束旧进程：taskkill /F /IM micro-manager.exe\n\n%v", cfg.Server.Port, err)
			}
			fatalUI("启动失败", msg)
		}
	}()

	systray.Run(func() { onTrayReady(cfg) }, nil)
}

func rescanAll(cfg *config.Config, st *store.Store, mgr *manager.Manager) {
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

// ---------- 托盘 ----------

func onTrayReady(cfg *config.Config) {
	systray.SetIcon(appIcon())
	systray.SetTitle("Micro Manager")
	systray.SetTooltip("微服务轻量管家 " + version)

	mOpen := systray.AddMenuItem("打开面板", "在浏览器中打开管理界面")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出面板（已启动的服务进程不受影响）")

	openBrowser(fmt.Sprintf("http://localhost:%d", cfg.Server.Port))
	go func() {
		for range mOpen.ClickedCh {
			openBrowser(fmt.Sprintf("http://localhost:%d", cfg.Server.Port))
		}
	}()
	go func() {
		for range mQuit.ClickedCh {
			systray.Quit()
		}
	}()
}

func openBrowser(url string) {
	// rundll32 不会闪黑窗口
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

// appIcon 运行时生成 64x64 渐变圆角图标（ICO 内嵌 PNG）
func appIcon() []byte {
	size := 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x-size/2), float64(y-size/2)
			d := dx*dx + dy*dy
			if d <= float64(size/2-2)*float64(size/2-2) {
				t := float64(y) / float64(size)
				img.Set(x, y, color.RGBA{
					R: uint8(37 + t*(124-37)),
					G: uint8(99 + t*(58-99)),
					B: uint8(235 + t*(142-235)),
					A: 255,
				})
			}
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil
	}

	ico := make([]byte, 22)
	ico[0], ico[1] = 0, 0 // reserved
	ico[2], ico[3] = 1, 0 // type: icon
	ico[4], ico[5] = 1, 0 // count
	ico[6] = byte(size)   // width
	ico[7] = byte(size)   // height
	ico[10], ico[11] = 1, 0  // planes
	ico[12], ico[13] = 32, 0 // bpp
	sz := uint32(pngBuf.Len())
	ico[14] = byte(sz)
	ico[15] = byte(sz >> 8)
	ico[16] = byte(sz >> 16)
	ico[17] = byte(sz >> 24)
	ico[18] = 22 // offset
	ico[19], ico[20], ico[21] = 0, 0, 0
	return append(ico, pngBuf.Bytes()...)
}

func printBannerTo(w interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(w, `
 __  __ _                __  __
|  \/  (_) ___ _ __ ___ |  \/  | __ _ _ __   __ _  __ _  ___ _ __
| |\/| | |/ __| '__/ _ \| |\/| |/ _`+"`"+` | '_ \ / _`+"`"+` |/ _`+"`"+` |/ _ \ '__|
| |  | | | (__| | | (_) | |  | | (_| | | | | (_| | (_| |  __/ |
|_|  |_|_|\___|_|  \___/|_|  |_|\__,_|_| |_|\__,_|\__, |\___|_|
                                                  |___/
  :: Java / Python / Node / Go Services Manager ::          (`+version+`)
`)
}

func setupLogging(cfg *config.Config) {
	f, err := os.OpenFile(filepath.Join(cfg.LogDir, "manager.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(f)
	}
	printBannerTo(f)
}

// ---------- UI 错误提示（无控制台模式用系统弹窗） ----------

func fatalUI(title, msg string) {
	log.Printf("[启动失败] %s: %s", title, msg)
	MessageBox(title, msg)
	os.Exit(1)
}

func MessageBox(title, text string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	mb := user32.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(title)
	b, _ := syscall.UTF16PtrFromString(text)
	mb.Call(0, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(t)), 0x40)
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
