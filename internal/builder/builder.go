package builder

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"sync"

	"microservice-manager/internal/ws"
)

// BuildServiceID 构建日志在 WS 中的伪服务 ID，前端复用日志终端展示
const BuildServiceID = "__build__"

type Builder struct {
	hub   *ws.Hub
	OnDone func(dir string, success bool)

	mu       sync.Mutex
	running  bool
	dir      string
	lastExit string
	cmd      *exec.Cmd
	lines    []string
}

func New(hub *ws.Hub) *Builder {
	return &Builder{hub: hub}
}

func (b *Builder) Start(dir string) error {
	if dir == "" {
		return errors.New("请填写项目目录（含 pom.xml）")
	}
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return errors.New("已有构建任务在运行，请先等待或停止")
	}
	b.running = true
	b.dir = dir
	b.lines = nil
	b.mu.Unlock()

	cmd := exec.Command("cmd", "/c", "mvn", "package", "-DskipTests")
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		b.setDone(false, err.Error())
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		b.setDone(false, err.Error())
		return err
	}
	if err := cmd.Start(); err != nil {
		b.setDone(false, err.Error())
		return fmt.Errorf("启动 mvn 失败（确认已安装 Maven 并加入 PATH）: %w", err)
	}
	b.mu.Lock()
	b.cmd = cmd
	b.mu.Unlock()

	b.append(fmt.Sprintf("[builder] 开始构建: %s", dir))
	go func() {
		done := make(chan struct{})
		go func() { b.pipe(stdout); close(done) }()
		b.pipe(stderr)
		<-done
		err := cmd.Wait()
		msg := "构建成功"
		if err != nil {
			msg = "构建失败: " + err.Error()
		}
		b.append("[builder] " + msg)
		b.setDone(err == nil, msg)
		if b.OnDone != nil {
			b.OnDone(dir, err == nil)
		}
	}()
	return nil
}

func (b *Builder) pipe(r interface{ Read([]byte) (int, error) }) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		b.append(sc.Text())
	}
}

func (b *Builder) append(line string) {
	b.mu.Lock()
	b.lines = append(b.lines, line+"\n")
	if len(b.lines) > 2000 {
		b.lines = b.lines[len(b.lines)-2000:]
	}
	b.mu.Unlock()
	b.hub.BroadcastTo(BuildServiceID, map[string]interface{}{"type": "log", "serviceId": BuildServiceID, "data": line + "\n"})
}

func (b *Builder) setDone(success bool, msg string) {
	b.mu.Lock()
	b.running = false
	b.lastExit = msg
	b.mu.Unlock()
	state := "done"
	if !success {
		state = "failed"
	}
	b.hub.Broadcast(map[string]interface{}{"type": "build", "state": state, "message": msg})
}

func (b *Builder) Stop() error {
	b.mu.Lock()
	cmd := b.cmd
	running := b.running
	b.mu.Unlock()
	if !running || cmd == nil || cmd.Process == nil {
		return errors.New("没有正在运行的构建任务")
	}
	exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
	return nil
}

func (b *Builder) Status() (running bool, dir string, lastExit string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running, b.dir, b.lastExit
}

func (b *Builder) Tail(n int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || n > len(b.lines) {
		n = len(b.lines)
	}
	out := make([]string, n)
	copy(out, b.lines[len(b.lines)-n:])
	return out
}
