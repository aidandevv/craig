package rules

import "testing"

func TestDefaultRuleSetParsesAndCompiles(t *testing.T) {
	set, err := DefaultRuleSet()
	if err != nil {
		t.Fatalf("default rules: %v", err)
	}
	compiled, err := Compile(set, Deps{})
	if err != nil {
		t.Fatalf("compile default rules: %v", err)
	}
	if len(compiled.Detectors) == 0 {
		t.Error("no offline detectors")
	}
	if len(compiled.Unavailable) == 0 {
		t.Error("no image rules to report unavailable")
	}
}

func TestDefaultRuleSetCoversEveryPreset(t *testing.T) {
	set, err := DefaultRuleSet()
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, rule := range set.Rules {
		if rule.Match.Preset != "" {
			used[rule.Match.Preset] = true
		}
	}
	for name := range Presets() {
		if !used[name] {
			t.Errorf("preset %q has no default rule", name)
		}
	}
}
