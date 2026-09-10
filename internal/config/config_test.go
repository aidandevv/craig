package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeCreatesPrivateConfigAndStarterRules(t *testing.T) {
	home := t.TempDir()
	paths := DefaultPaths(home)
	cfg, err := Default(home)
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if err := Initialize(paths.Config, cfg); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	for _, path := range []string{paths.Config, paths.Rules} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s mode = %o, want 600", path, got)
		}
	}
	loaded, err := Load(paths.Config)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if loaded.Daemon.Token != cfg.Daemon.Token || loaded.Rules.File != paths.Rules {
		t.Errorf("loaded config = %+v, want token and rules path retained", loaded)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "craig-extension")); !os.IsNotExist(err) {
		t.Errorf("initialization should not create cache yet: %v", err)
	}
	if err := Initialize(paths.Config, cfg); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second initialization error = %v, want existing-file refusal", err)
	}
}

func TestLoadExpandsConfiguredEnvironmentKey(t *testing.T) {
	home := t.TempDir()
	paths := DefaultPaths(home)
	cfg, err := Default(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := Initialize(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_VISION_API_KEY", "vision-from-environment")
	loaded, err := Load(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.APIKeys.GoogleVision; got != "vision-from-environment" {
		t.Errorf("expanded key = %q, want environment value", got)
	}
}

func TestLoadRejectsUnsafeConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	bad := `daemon:
  port: 8765
  token: short
  log_level: info
api_keys: {}
vision: {monthly_unit_cap: 1}
cache: {location: cache.db, max_age_days: 1}
rules: {file: rules.yaml, auto_reload: true}
`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "token") {
		t.Errorf("Load error = %v, want token validation error", err)
	}
}

func TestNewTokenIs32BytesOfHex(t *testing.T) {
	token, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64 hex characters", len(token))
	}
}
