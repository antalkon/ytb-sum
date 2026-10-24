package config

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// KeychainService is the macOS Keychain service name holding the API key.
const KeychainService = "ytb-sum"

// LoadKeychain fills APIKey from the macOS Keychain when no key is set yet.
// It silently does nothing on other platforms or when no entry exists.
func (c *Config) LoadKeychain() {
	if c.APIKey != "" || runtime.GOOS != "darwin" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	args := []string{"find-generic-password", "-s", KeychainService, "-w"}
	if u := os.Getenv("USER"); u != "" {
		args = append(args, "-a", u)
	}
	out, err := exec.CommandContext(ctx, "security", args...).Output()
	if err != nil {
		return
	}
	if k := strings.TrimSpace(string(out)); k != "" {
		c.APIKey, c.APIKeySource = k, "macOS Keychain"
	}
}
