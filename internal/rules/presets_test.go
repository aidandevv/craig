package rules

import (
	"regexp"
	"strings"
	"testing"
)

func TestPresetsAreNonEmptyAndCompile(t *testing.T) {
	presets := Presets()
	if len(presets) == 0 {
		t.Fatal("no presets embedded")
	}
	for name, preset := range presets {
		if strings.TrimSpace(preset.Description) == "" {
			t.Errorf("preset %q has no description; the builder shows it to users", name)
		}
		if len(preset.Patterns) == 0 {
			t.Errorf("preset %q has no patterns", name)
		}
		for i, pattern := range preset.Patterns {
			if _, err := regexp.Compile(pattern); err != nil {
				t.Errorf("preset %q pattern %d (%q) does not compile: %v", name, i, pattern, err)
			}
		}
	}
}

func TestExpectedPresetsExist(t *testing.T) {
	want := []string{
		"payment_no_recourse",
		"urgency_pressure",
		"absentee_landlord",
		"contact_evasion_phrases",
		"deposit_before_viewing",
	}
	presets := Presets()
	for _, name := range want {
		if _, ok := presets[name]; !ok {
			t.Errorf("preset %q missing", name)
		}
	}
}

func TestLookupPresetRejectsUnknownName(t *testing.T) {
	if _, err := LookupPreset("payment_no_recourse"); err != nil {
		t.Errorf("known preset rejected: %v", err)
	}
	_, err := LookupPreset("no_such_preset")
	if err == nil {
		t.Fatal("expected an error for an unknown preset")
	}
	if !strings.Contains(err.Error(), "no_such_preset") {
		t.Errorf("error %q does not name the missing preset", err)
	}
}

// The presets must actually catch the scams they claim to.
func TestPresetsMatchRealisticScamText(t *testing.T) {
	cases := []struct{ preset, text string }{
		{"payment_no_recourse", "please send the deposit by western union"},
		{"payment_no_recourse", "i can only accept gift cards for the first month"},
		{"payment_no_recourse", "payment via zelle only, no exceptions"},
		{"urgency_pressure", "MUST GO TODAY, several people interested"},
		{"urgency_pressure", "first come first served, act now"},
		{"absentee_landlord", "i am currently out of the country on mission work"},
		{"absentee_landlord", "renting sight unseen is fine, i will ship the keys"},
		{"contact_evasion_phrases", "email only please, no calls"},
		{"deposit_before_viewing", "send a deposit to hold the unit before viewing"},
	}
	for _, tc := range cases {
		preset, err := LookupPreset(tc.preset)
		if err != nil {
			t.Fatalf("lookup %s: %v", tc.preset, err)
		}
		matched := false
		for _, pattern := range preset.Patterns {
			re := regexp.MustCompile("(?i)" + pattern)
			if re.MatchString(tc.text) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("preset %q did not match %q", tc.preset, tc.text)
		}
	}
}

// A preset that fires on ordinary listing prose is worse than no preset.
func TestPresetsDoNotMatchInnocuousText(t *testing.T) {
	innocuous := []string{
		"Bright two bedroom with a dishwasher and in-unit laundry.",
		"Available from the first of next month. Text or call to arrange a viewing.",
		"Deposit is one month's rent, due at lease signing after you see the place.",
		"Close to the park; street parking is usually easy.",
	}
	for name, preset := range Presets() {
		for _, pattern := range preset.Patterns {
			re := regexp.MustCompile("(?i)" + pattern)
			for _, text := range innocuous {
				if re.MatchString(text) {
					t.Errorf("preset %q pattern %q false-positives on %q", name, pattern, text)
				}
			}
		}
	}
}
