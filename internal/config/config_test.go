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
	if loaded.Daemon.ID != cfg.Daemon.ID || len(loaded.Daemon.ID) != 32 {
		t.Errorf("loaded daemon ID = %q, want generated 16-byte hex identifier", loaded.Daemon.ID)
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

func TestEnsureDaemonIDMigratesOlderConfigWithoutExpandingVisionEnvironmentReference(t *testing.T) {
	home := t.TempDir()
	paths := DefaultPaths(home)
	cfg, err := Default(home)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Daemon.ID = ""
	if err := Initialize(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_VISION_API_KEY", "should-stay-out-of-config")
	id, err := EnsureDaemonID(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 32 {
		t.Errorf("daemon ID length = %d, want 32", len(id))
	}
	again, err := EnsureDaemonID(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if again != id {
		t.Errorf("daemon ID changed from %q to %q", id, again)
	}
	raw, err := os.ReadFile(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "should-stay-out-of-config") || !strings.Contains(string(raw), "${GOOGLE_VISION_API_KEY}") {
		t.Errorf("migration rewrote Vision environment reference: %s", raw)
	}
}

func TestSetGoogleVisionAPIKeyKeepsConfigPrivateAndReturnsSafeVisionSettings(t *testing.T) {
	home := t.TempDir()
	paths := DefaultPaths(home)
	cfg, err := Default(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := Initialize(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	settings, err := SetGoogleVisionAPIKey(paths.Config, "  direct-vision-key  ")
	if err != nil {
		t.Fatal(err)
	}
	if settings.MonthlyUnitCap != cfg.Vision.MonthlyUnitCap || settings.UseADC != cfg.Vision.UseADC {
		t.Errorf("returned Vision settings = %+v, want existing non-secret settings", settings)
	}
	info, err := os.Stat(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("config mode = %o, want 600", got)
	}
	loaded, err := Load(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKeys.GoogleVision != "direct-vision-key" {
		t.Errorf("saved Vision key = %q, want trimmed direct value", loaded.APIKeys.GoogleVision)
	}
	if _, err := SetGoogleVisionAPIKey(paths.Config, " "); err == nil {
		t.Error("empty Vision key was accepted")
	}
}
