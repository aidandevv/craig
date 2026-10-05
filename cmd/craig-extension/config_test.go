package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aidandevv/craig/internal/config"
)

func TestConfigInitAndTokenCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	var initOutput bytes.Buffer
	if code := runConfig([]string{"init", "--config", path}, &initOutput); code != exitOK {
		t.Fatalf("config init exit %d: %s", code, initOutput.String())
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "rules.yaml")); err != nil {
		t.Fatalf("starter rules missing: %v", err)
	}
	var tokenOutput bytes.Buffer
	if code := runConfig([]string{"token", "--config", path}, &tokenOutput); code != exitOK {
		t.Fatalf("config token exit %d: %s", code, tokenOutput.String())
	}
	if got := strings.TrimSpace(tokenOutput.String()); got != cfg.Daemon.Token {
		t.Errorf("token output = %q, want generated token", got)
	}
}

func TestConfigInitRefusesExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if code := runConfig([]string{"init", "--config", path}, &output); code != exitRuntime || !strings.Contains(output.String(), "already exists") {
		t.Errorf("config init = exit %d, %q", code, output.String())
	}
}
