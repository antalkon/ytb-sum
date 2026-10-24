package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("lang: en\nquality: fast\napi_key: sk-file-123456789\n"), 0o644)

	c := Defaults()
	warn, err := c.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn, "chmod 600") {
		t.Errorf("expected permission warning, got %q", warn)
	}
	if c.Lang != "en" || c.ResolvedModel() != QualityPresets["fast"] || c.Length != DefaultLength {
		t.Errorf("file layer: %+v", c)
	}
	env := map[string]string{"OPENROUTER_API_KEY": "sk-env-123456789", "YTB_SUM_MODEL": "x/y"}
	c.LoadEnv(func(k string) string { return env[k] })
	if c.APIKey != "sk-env-123456789" || c.APIKeySource != "env OPENROUTER_API_KEY" || c.ResolvedModel() != "x/y" {
		t.Errorf("env layer: %+v", c)
	}
}

func TestMissingFileIsFine(t *testing.T) {
	c := Defaults()
	if _, err := c.LoadFile(filepath.Join(t.TempDir(), "nope.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestValidate(t *testing.T) {
	c := Defaults()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Quality = "ultra"
	if c.Validate() == nil {
		t.Error("unknown quality should fail")
	}
	c.Model = "some/model" // explicit model makes quality irrelevant
	if err := c.Validate(); err != nil {
		t.Error(err)
	}
	c.Length = "huge"
	if c.Validate() == nil {
		t.Error("unknown length should fail")
	}
}

func TestMaskKey(t *testing.T) {
	if got := MaskKey("sk-or-v1-abcdefghijklmnop"); got != "sk-or-…mnop" {
		t.Errorf("MaskKey = %q", got)
	}
	if MaskKey("") != "(not set)" || MaskKey("short") != "****" {
		t.Error("MaskKey edge cases")
	}
}
