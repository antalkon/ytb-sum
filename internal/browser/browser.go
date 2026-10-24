// Package browser opens files in the user's default web browser.
package browser

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// Open opens path with the system handler (the default browser for .html).
// It returns once the handler has been launched.
func Open(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", abs)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() //nolint:errcheck // reap the launcher; its result is irrelevant
	return nil
}
