package executil

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Command 同 exec.Command，但在 Windows 上不弹出控制台黑框
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}
