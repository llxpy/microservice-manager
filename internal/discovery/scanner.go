package discovery

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
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
		if existing, _ := st.GetByPath(jar); existing != nil {
			res.Existing++
			continue
		}
		sv := &store.Service{
			ID:          name + "@default",
			Name:        name,
			Group:       "default",
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
			res.New = append(res.New, sv)
		}
	}
	return res, nil
}

func guessName(base string) string {
	n := strings.TrimSuffix(base, filepath.Ext(base))
	re := regexp.MustCompile(`[-_]v?\d[\d._]*$`)
	return re.ReplaceAllString(n, "")
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
