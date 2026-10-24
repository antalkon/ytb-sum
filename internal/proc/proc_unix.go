//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

// killTree runs cmd in its own process group so cancellation kills any
// children too (yt-dlp may spawn ffmpeg).
func killTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
