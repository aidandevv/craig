package rules

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed presets.yaml
var presetsYAML []byte

//go:embed default_rules.yaml
var defaultRulesYAML []byte

// Preset is a named group of patterns shipped with the binary. Presets use the
// same machinery as custom rules, so a user can inspect one and fork it.
type Preset struct {
	Description string   `yaml:"description"`
	Patterns    []string `yaml:"patterns"`
}

var (
	presetsOnce sync.Once
	presets     map[string]Preset
)

// Presets returns the embedded library. A malformed presets.yaml is a build
// defect, not a user error, so it panics rather than degrading silently.
func Presets() map[string]Preset {
	presetsOnce.Do(func() {
		if err := yaml.Unmarshal(presetsYAML, &presets); err != nil {
			panic("rules: embedded presets.yaml is malformed: " + err.Error())
		}
	})
	return presets
}

// LookupPreset resolves a preset by name, listing the alternatives when the
// name is wrong — a misremembered preset name is the likeliest authoring error.
func LookupPreset(name string) (Preset, error) {
	if preset, ok := Presets()[name]; ok {
		return preset, nil
	}
	available := make([]string, 0, len(Presets()))
	for key := range Presets() {
		available = append(available, key)
	}
	sort.Strings(available)
	return Preset{}, fmt.Errorf("unknown preset %q (available: %s)", name, strings.Join(available, ", "))
}

// DefaultRuleSet returns the embedded starter rules, allowing useful offline
// analysis before a user has created a configuration file.
func DefaultRuleSet() (RuleSet, error) {
	set, err := Parse(defaultRulesYAML)
	if err != nil {
		return RuleSet{}, fmt.Errorf("embedded default rules: %w", err)
	}
	return set, nil
}
