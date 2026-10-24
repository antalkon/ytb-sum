//go:build windows

package proc

import (
	"os/exec"
	"strconv"
)

// killTree makes cancellation kill cmd and its children via taskkill /T.
func killTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		if err := kill.Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
