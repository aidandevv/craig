package rules

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
)

// Check names available to a contact_check rule.
const (
	CheckObfuscatedDigits   = "obfuscated_digits"
	CheckSpelledOutDigits   = "spelled_out_digits"
	CheckEmailObfuscation   = "email_obfuscation"
	CheckRelayOnly          = "relay_only"
	CheckAppOnly            = "app_only"
	CheckDirectPhonePresent = "direct_phone_present"
	CheckDirectEmailPresent = "direct_email_present"
)

var (
	// A run of single digits split by spaces or unusual separators. Requires
	// seven such digits, so ordinary formatting like 510-555-1234 and street
	// numbers do not trip it.
	reObfuscatedDigits = regexp.MustCompile(`(?:\d[\s\-–—_.·]{1,3}){6,}\d`)

	// Three or more spelled-out digits in sequence. Two is common in prose
	// ("two bedroom, one bath"); three in a row is someone spelling a number out.
	digitWord              = `(?:zero|one|two|three|four|five|six|seven|eight|nine|oh)`
	reSpelledOutDigits     = regexp.MustCompile(`(?i)\b` + digitWord + `[\s\-]+` + digitWord + `[\s\-]+` + digitWord + `\b`)
	reEmailObfuscationWord = regexp.MustCompile(`(?i)\b\S+\s*(?:\(|\[)?\s*(?:at|@)\s*(?:\)|\])?\s*\S+\s*(?:\(|\[)?\s*dot\s*(?:\)|\])?\s*(?:com|net|org|co\.uk)\b`)
)

// checkFunc reports whether a check fires, and a short human explanation.
// needsText identifies checks that cannot run when the selected listing scope
// was not supplied.
type checkFunc struct {
	needsText bool
	evaluate  func(listing domain.Listing, text string) (string, bool)
}

var checkRegistry = map[string]checkFunc{
	CheckObfuscatedDigits: {
		needsText: true,
		evaluate: func(_ domain.Listing, text string) (string, bool) {
			if m := reObfuscatedDigits.FindString(text); m != "" {
				return fmt.Sprintf("digits split to evade detection (%q)", strings.TrimSpace(m)), true
			}
			return "", false
		},
	},
	CheckSpelledOutDigits: {
		needsText: true,
		evaluate: func(_ domain.Listing, text string) (string, bool) {
			if m := reSpelledOutDigits.FindString(text); m != "" {
				return fmt.Sprintf("a number spelled out in words (%q)", strings.TrimSpace(m)), true
			}
			return "", false
		},
	},
	CheckEmailObfuscation: {
		needsText: true,
		evaluate: func(_ domain.Listing, text string) (string, bool) {
			if m := reEmailObfuscationWord.FindString(text); m != "" {
				return fmt.Sprintf("an email address written to evade detection (%q)", strings.TrimSpace(m)), true
			}
			return "", false
		},
	},
	CheckRelayOnly: {
		evaluate: func(l domain.Listing, _ string) (string, bool) {
			return "the seller is reachable only through the marketplace relay", l.Contact.RelayOnly
		},
	},
	CheckAppOnly: {
		evaluate: func(l domain.Listing, _ string) (string, bool) {
			return "the seller is reachable only through in-app messaging", l.Contact.AppOnly
		},
	},
	CheckDirectPhonePresent: {
		evaluate: func(l domain.Listing, _ string) (string, bool) {
			return "a direct phone number is listed", strings.TrimSpace(l.Contact.Phone) != ""
		},
	},
	CheckDirectEmailPresent: {
		evaluate: func(l domain.Listing, _ string) (string, bool) {
			return "a direct email address is listed", strings.TrimSpace(l.Contact.Email) != ""
		},
	},
}

func knownCheck(name string) bool {
	_, ok := checkRegistry[name]
	return ok
}

func allChecks() []string {
	names := make([]string, 0, len(checkRegistry))
	for name := range checkRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// contactCheck looks for a seller hiding a verifiable contact route, or — for
// green rules — confirms one is present.
type contactCheck struct {
	name       string
	weight     float64
	hard       bool
	scope      string
	requireAll bool
	checks     []string
}

func newContactCheck(name string, r Rule, weight float64) (detect.Detector, error) {
	for _, check := range r.Checks {
		if !knownCheck(check) {
			return nil, fmt.Errorf("rule %q: unknown check %q (available: %s)",
				name, check, strings.Join(allChecks(), ", "))
		}
	}
	return &contactCheck{
		name: name, weight: weight, hard: r.Hard, scope: r.Scope,
		requireAll: r.RequireAll, checks: r.Checks,
	}, nil
}

func (c *contactCheck) Name() string { return c.name }

func (c *contactCheck) Evaluate(_ context.Context, listing domain.Listing) (domain.SignalResult, error) {
	text, present := scopeText(listing, c.scope)
	for _, check := range c.checks {
		if checkRegistry[check].needsText && !present {
			return domain.SignalResult{
				Name:    c.name,
				Skipped: domain.SkipMissingField,
				Details: []string{fmt.Sprintf("%s: listing has no %s text", c.name, c.scope)},
			}, nil
		}
	}

	var reasons []string
	for _, check := range c.checks {
		if reason, fired := checkRegistry[check].evaluate(listing, text); fired {
			reasons = append(reasons, reason)
		}
	}

	matched := len(reasons) > 0
	if c.requireAll {
		matched = len(reasons) == len(c.checks)
	}
	if !matched {
		return domain.SignalResult{Name: c.name}, nil
	}

	return domain.SignalResult{
		Name:    c.name,
		Risk:    detect.Clamp(c.weight),
		Hard:    c.hard,
		Flags:   []string{c.name},
		Details: []string{fmt.Sprintf("%s: %s", c.name, strings.Join(reasons, "; "))},
	}, nil
}
