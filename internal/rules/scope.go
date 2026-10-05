package rules

import (
	"strings"

	"github.com/aidandevv/craig/internal/domain"
)

// scopeText returns the listing text a scope selects. The boolean reports
// whether that text exists at all: a rule with nothing to read is not
// evaluated, never a silent pass, because "we could not check" and "we checked
// and it was fine" must not look the same to the user.
func scopeText(l domain.Listing, scope string) (string, bool) {
	switch scope {
	case ScopeTitle:
		return l.Title, strings.TrimSpace(l.Title) != ""
	case ScopeDescription:
		return l.Description, strings.TrimSpace(l.Description) != ""
	case ScopeCaption:
		joined := strings.Join(l.Captions, "\n")
		return joined, strings.TrimSpace(joined) != ""
	case ScopeWholePost:
		parts := make([]string, 0, 2+len(l.Captions))
		for _, part := range append([]string{l.Title, l.Description}, l.Captions...) {
			if strings.TrimSpace(part) != "" {
				parts = append(parts, part)
			}
		}
		joined := strings.Join(parts, "\n")
		return joined, joined != ""
	}
	return "", false
}
