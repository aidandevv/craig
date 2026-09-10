// Package config owns the daemon's on-disk configuration. It deliberately
// keeps secrets in a mode-0600 file and expands environment references only at
// load time, so an API key can stay outside the file in a shell profile or
// password manager integration.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/aidandevv/craig-extension/internal/rules"
)

const (
	defaultPort       = 8765
	defaultMonthlyCap = 999
	defaultMaxAgeDays = 30
)

// Config is the complete daemon configuration. The exported fields have YAML
// names only; callers should use SafeSummary rather than serializing this type
// because APIKeys are secrets.
type Config struct {
	Daemon  Daemon  `yaml:"daemon"`
	APIKeys APIKeys `yaml:"api_keys"`
	Vision  Vision  `yaml:"vision"`
	Cache   Cache   `yaml:"cache"`
	Rules   Rules   `yaml:"rules"`
}

type Daemon struct {
	Port     int    `yaml:"port"`
	Token    string `yaml:"token"`
	LogLevel string `yaml:"log_level"`
}

type APIKeys struct {
	GoogleVision string `yaml:"google_vision"`
}

type Vision struct {
	MonthlyUnitCap int  `yaml:"monthly_unit_cap"`
	UseADC         bool `yaml:"use_adc"`
}

type Cache struct {
	Location   string `yaml:"location"`
	MaxAgeDays int    `yaml:"max_age_days"`
}

type Rules struct {
	File       string `yaml:"file"`
	AutoReload bool   `yaml:"auto_reload"`
}

// Paths contains the conventional locations for a local installation. Keeping
// this explicit makes config initialization testable without reading a user's
// real home directory.
type Paths struct {
	Config string
	Rules  string
	Cache  string
}

// DefaultPaths returns the XDG-style locations described in the project
// design. The desktop daemon follows these locations on every OS so that the
// extension instructions do not vary by platform.
func DefaultPaths(home string) Paths {
	return Paths{
		Config: filepath.Join(home, ".config", "craig-extension", "config.yaml"),
		Rules:  filepath.Join(home, ".config", "craig-extension", "rules.yaml"),
		Cache:  filepath.Join(home, ".local", "share", "craig-extension", "cache.db"),
	}
}

// DefaultPath locates config.yaml for the active user.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return DefaultPaths(home).Config, nil
}

// Default returns an initialized configuration with a fresh bearer token.
// The token is generated before writing so callers can safely persist the
// result using Initialize or Write.
func Default(home string) (Config, error) {
	paths := DefaultPaths(home)
	token, err := NewToken()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Daemon:  Daemon{Port: defaultPort, Token: token, LogLevel: "info"},
		APIKeys: APIKeys{GoogleVision: "${GOOGLE_VISION_API_KEY}"},
		Vision:  Vision{MonthlyUnitCap: defaultMonthlyCap},
		Cache:   Cache{Location: paths.Cache, MaxAgeDays: defaultMaxAgeDays},
		Rules:   Rules{File: paths.Rules, AutoReload: true},
	}, nil
}

// NewToken returns a 32-byte bearer token represented as hexadecimal.
func NewToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate bearer token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// Initialize creates a config file and its editable starter rules file. It
// refuses to overwrite either file: regenerating credentials silently would
// disconnect an existing extension, and overwriting rules would lose work.
func Initialize(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("config path is required")
	}
	if strings.TrimSpace(cfg.Daemon.Token) == "" {
		return errors.New("daemon token is required")
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists at %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect config path: %w", err)
	}
	if _, err := os.Stat(cfg.Rules.File); err == nil {
		return fmt.Errorf("rules file already exists at %s", cfg.Rules.File)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect rules path: %w", err)
	}
	if err := WriteFileSecure(cfg.Rules.File, rules.DefaultRulesYAML()); err != nil {
		return fmt.Errorf("write starter rules: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := WriteFileSecure(path, data); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Load reads, expands and validates configuration from disk. Environment
// indirection such as ${GOOGLE_VISION_API_KEY} is expanded only here, after the
// on-disk configuration has been parsed.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.expand(); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) expand() error {
	var err error
	if c.APIKeys.GoogleVision, err = expandEnvironment(c.APIKeys.GoogleVision); err != nil {
		return fmt.Errorf("expand api_keys.google_vision: %w", err)
	}
	for _, item := range []struct {
		name  string
		value *string
	}{
		{"cache.location", &c.Cache.Location},
		{"rules.file", &c.Rules.File},
	} {
		if *item.value, err = expandPath(*item.value); err != nil {
			return fmt.Errorf("expand %s: %w", item.name, err)
		}
	}
	return nil
}

func expandEnvironment(value string) (string, error) {
	var missing string
	expanded := os.Expand(value, func(name string) string {
		found, ok := os.LookupEnv(name)
		if !ok {
			missing = name
			return ""
		}
		return found
	})
	if missing != "" {
		// A missing API key is allowed; it keeps Vision explicitly disabled.
		return "", nil
	}
	return expanded, nil
}

func expandPath(value string) (string, error) {
	expanded, err := expandEnvironment(value)
	if err != nil {
		return "", err
	}
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
	}
	return expanded, nil
}

// Validate ensures a loaded config is both safe to bind and usable by the
// daemon. API keys remain optional by design.
func (c Config) Validate() error {
	if c.Daemon.Port < 1 || c.Daemon.Port > 65535 {
		return fmt.Errorf("daemon.port must be in [1,65535], got %d", c.Daemon.Port)
	}
	token, err := hex.DecodeString(c.Daemon.Token)
	if err != nil || len(token) != 32 {
		return errors.New("daemon.token must be a 32-byte hexadecimal token")
	}
	switch c.Daemon.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("daemon.log_level must be debug, info, warn, or error")
	}
	if c.Vision.MonthlyUnitCap < 1 {
		return fmt.Errorf("vision.monthly_unit_cap must be at least 1")
	}
	if strings.TrimSpace(c.Cache.Location) == "" {
		return errors.New("cache.location is required")
	}
	if c.Cache.MaxAgeDays < 1 {
		return fmt.Errorf("cache.max_age_days must be at least 1")
	}
	if strings.TrimSpace(c.Rules.File) == "" {
		return errors.New("rules.file is required")
	}
	return nil
}

// WriteFileSecure atomically writes a private configuration or rules file.
// The parent directory is private too, and the replacement file is chmodded
// before rename so an API key is never briefly exposed with a process-default
// mode.
func WriteFileSecure(path string, data []byte) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".craig-extension-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
