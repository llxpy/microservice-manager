package discovery

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"microservice-manager/internal/store"
)

var defaultJavaOpts = "-Xms96m -Xmx128m -XX:+UseSerialGC"

type Result struct {
	New      []*store.Service
	Existing int
}

func Scan(scanDir string, st *store.Store) (*Result, error) {
	res := &Result{}
	if scanDir == "" {
		return res, nil
	}
	existing, _ := st.GetAll()
	byPath := map[string]bool{}
	byWorkDir := map[string]bool{}
	for _, sv := range existing {
		byPath[sv.Path] = true
		if sv.WorkDir != "" {
			byWorkDir[strings.ToLower(sv.WorkDir)] = true
		}
	}

	var jars []string
	filepath.Walk(scanDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".jar") &&
			!strings.Contains(strings.ToLower(filepath.Base(path)), "sources") {
			jars = append(jars, path)
		}
		return nil
	})

	for _, jar := range jars {
		port, name, runnable := parseJarMeta(jar)
		if !runnable {
			continue // 跳过普通库 jar（无 BOOT-INF，无法 java -jar 运行）
		}
		if name == "" {
			name = guessName(filepath.Base(jar))
		}
		group := projectGroup(scanDir, jar)
		if byPath[jar] {
			res.Existing++
			continue
		}
		sv := &store.Service{
			ID:          name + "@" + group,
			Name:        name,
			Group:       group,
			Type:        "jar",
			Path:        jar,
			WorkDir:     filepath.Dir(jar),
			JavaOpts:    defaultJavaOpts,
			Port:        port,
			AutoRestart: true,
			Enabled:     true,
		}
		if port > 0 {
			sv.HealthURL = fmt.Sprintf("http://localhost:%d/actuator/health", port)
		}
		if err := st.Upsert(sv); err == nil {
			byPath[jar] = true
			res.New = append(res.New, sv)
		}
	}

	scanPolyglotProjects(scanDir, st, res, byPath, byWorkDir)
	return res, nil
}

// scanPolyglotProjects 识别扫描目录本身及一级子目录中的 Python / Node / Go 项目
func scanPolyglotProjects(scanDir string, st *store.Store, res *Result, byPath, byWorkDir map[string]bool) {
	candidates := []string{scanDir}
	if entries, err := os.ReadDir(scanDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			switch strings.ToLower(e.Name()) {
			case "node_modules", "target", "build", "dist", "out", "venv", "__pycache__", "docs", "logs", "libs":
				continue
			}
			candidates = append(candidates, filepath.Join(scanDir, e.Name()))
		}
	}
	for _, dir := range candidates {
		sv := detectProject(dir)
		if sv == nil {
			continue
		}
		if byPath[sv.Path] || byWorkDir[strings.ToLower(dir)] {
			res.Existing++
			continue
		}
		if err := st.Upsert(sv); err == nil {
			byPath[sv.Path] = true
			byWorkDir[strings.ToLower(dir)] = true
			res.New = append(res.New, sv)
		}
	}
}

var pythonEntries = []string{"main.py", "app.py", "server.py", "run.py", "manage.py"}
var skipDirNames = map[string]bool{
	"node_modules": true, "target": true, "build": true, "dist": true, "out": true,
	"venv": true, "__pycache__": true, "docs": true, "logs": true, "libs": true,
}

// detectProject 通过特征文件识别项目类型与启动命令
func detectProject(dir string) *store.Service {
	// Python
	for _, entry := range pythonEntries {
		if fileExists(filepath.Join(dir, entry)) {
			return newDiscovered(dir, "python", pythonCommand(dir, entry), portFromAppYml(dir))
		}
	}
	// Node
	if fileExists(filepath.Join(dir, "package.json")) {
		if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
				Main    string            `json:"main"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				if pkg.Scripts != nil && pkg.Scripts["start"] != "" {
					return newDiscovered(dir, "node", "npm start", 0)
				}
				main := pkg.Main
				if main == "" {
					main = "index.js"
				}
				if fileExists(filepath.Join(dir, main)) {
					return newDiscovered(dir, "node", "node "+main, 0)
				}
			}
		}
	}
	// Go
	if fileExists(filepath.Join(dir, "main.go")) || fileExists(filepath.Join(dir, "go.mod")) {
		return newDiscovered(dir, "go", "go run .", 0)
	}
	return nil
}

func newDiscovered(dir, typ, command string, port int) *store.Service {
	name := filepath.Base(dir)
	sv := &store.Service{
		ID:          name + "@" + name,
		Name:        name,
		Group:       name,
		Type:        typ,
		Path:        dir,
		WorkDir:     dir,
		Command:     command,
		Port:        port,
		AutoRestart: true,
		Enabled:     true,
	}
	if port > 0 {
		sv.HealthURL = fmt.Sprintf("http://localhost:%d/actuator/health", port)
	}
	return sv
}

func portFromAppYml(dir string) int {
	if data, err := os.ReadFile(filepath.Join(dir, "application.yml")); err == nil {
		port, _ := parseYml(string(data))
		return port
	}
	return 0
}

// pythonCommand 为 Python 项目挑选解释器：venv > conda 环境声明 > 系统 python
func pythonCommand(dir, entry string) string {
	for _, v := range []string{".venv", "venv"} {
		py := filepath.Join(dir, v, "Scripts", "python.exe")
		if fileExists(py) {
			return py + " " + entry
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "environment.yml")); err == nil {
		var ef struct {
			Name string `yaml:"name"`
		}
		if yaml.Unmarshal(data, &ef) == nil && ef.Name != "" {
			if py := condaEnvPython(ef.Name); py != "" {
				return py + " " + entry
			}
		}
	}
	return "python " + entry
}

func condaEnvPython(env string) string {
	if out, err := exec.Command("conda", "env", "list", "--json").Output(); err == nil {
		var jl struct {
			Envs []string `json:"envs"`
		}
		if json.Unmarshal(out, &jl) == nil {
			for _, e := range jl.Envs {
				if strings.EqualFold(filepath.Base(e), env) {
					return filepath.Join(e, "python.exe")
				}
			}
			return ""
		}
	}
	// conda 不在 PATH 时探测常见安装位置
	for _, root := range condaCommonRoots() {
		py := filepath.Join(root, "envs", env, "python.exe")
		if fileExists(py) {
			return py
		}
	}
	return ""
}

func condaCommonRoots() []string {
	roots := []string{}
	if hf := os.Getenv("USERPROFILE"); hf != "" {
		roots = append(roots, filepath.Join(hf, "miniconda3"), filepath.Join(hf, "anaconda3"))
	}
	roots = append(roots, `D:\miniconda3`, `D:\anaconda3`, `C:\ProgramData\miniconda3`, `C:\ProgramData\anaconda3`)
	return roots
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// projectGroup 分组 = 项目名：扫描目录本身是项目（有 pom.xml/.git 等特征）则用其目录名；
// 否则是多项目容器目录，用 jar 所属的一级子目录名。
func projectGroup(scanDir, jarPath string) string {
	if isProjectRoot(scanDir) {
		return filepath.Base(scanDir)
	}
	rel, err := filepath.Rel(scanDir, filepath.Dir(jarPath))
	if err != nil || rel == "." {
		return filepath.Base(scanDir)
	}
	parts := strings.SplitN(rel, string(os.PathSeparator), 2)
	return parts[0]
}

func isProjectRoot(dir string) bool {
	for _, m := range []string{"pom.xml", "build.gradle", "settings.gradle", "package.json", "go.mod", "main.py", "app.py", ".git"} {
		if fileExists(filepath.Join(dir, m)) {
			return true
		}
	}
	return false
}

var reSnapshot = regexp.MustCompile(`[-._]?SNAPSHOT$`)
var reVersion = regexp.MustCompile(`[-_]?v?\d+(\.\d+)*([-.]?SNAPSHOT)?$`)

func guessName(base string) string {
	n := strings.TrimSuffix(base, filepath.Ext(base))
	n = reVersion.ReplaceAllString(n, "")
	n = reSnapshot.ReplaceAllString(n, "")
	return strings.Trim(n, "-_.")
}

func parseJarMeta(jarPath string) (port int, name string, runnable bool) {
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return 0, "", false
	}
	defer zr.Close()
	runnable = false
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "BOOT-INF/") {
			runnable = true // Spring Boot fat jar 才能用 java -jar 运行
		}
		base := strings.ToLower(f.Name)
		if base == "boot-inf/classes/application.yml" || base == "boot-inf/classes/application.yaml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			port, name := parseYml(string(data))
			return port, name, true
		}
		if base == "boot-inf/classes/application.properties" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			port, name := parseProperties(string(data))
			return port, name, true
		}
	}
	if !runnable {
		return 0, "", false
	}
	return 0, "", true
}

func parseYml(content string) (port int, name string) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return 0, ""
	}
	if srv, ok := doc["server"].(map[string]interface{}); ok {
		if p, ok := toInt(srv["port"]); ok {
			port = p
		}
	}
	if spring, ok := doc["spring"].(map[string]interface{}); ok {
		if app, ok := spring["application"].(map[string]interface{}); ok {
			if n, ok := app["name"].(string); ok {
				name = n
			}
		}
	}
	return port, name
}

func parseProperties(content string) (port int, name string) {
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "server.port":
			if p, err := strconv.Atoi(val); err == nil {
				port = p
			}
		case "spring.application.name":
			name = val
		}
	}
	return port, name
}

func toInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case float64:
		return int(t), true
	case string:
		s := strings.TrimSpace(t)
		// handle ${PORT:8080} placeholders
		if strings.HasPrefix(s, "${") && strings.Contains(s, ":") {
			inner := strings.TrimSuffix(strings.TrimPrefix(s, "${"), "}")
			idx := strings.Index(inner, ":")
			if idx >= 0 {
				s = strings.TrimSpace(inner[idx+1:])
			}
		}
		if p, err := strconv.Atoi(s); err == nil {
			return p, true
		}
	}
	return 0, false
}
