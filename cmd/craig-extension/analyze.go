package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aidandevv/craig-extension/internal/detect"
	"github.com/aidandevv/craig-extension/internal/domain"
	"github.com/aidandevv/craig-extension/internal/risk"
	"github.com/aidandevv/craig-extension/internal/rules"
)

const (
	exitOK      = 0
	exitUsage   = 1
	exitRuleSet = 2
	exitRuntime = 3
)

// runAnalyze is parameterized I/O so its exit behavior can be tested without
// starting a subprocess.
func runAnalyze(args []string, stdin io.Reader, stdout io.Writer) int {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(stdout)
	data := fs.String("data", "", "listing JSON, inline")
	file := fs.String("file", "", `path to listing JSON, or "-" for stdin`)
	rulesPath := fs.String("rules", "", "path to rules.yaml (default: embedded rule set)")
	asJSON := fs.Bool("json", false, "emit the assessment as JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stdout, "error: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return exitUsage
	}
	raw, err := readListing(*data, *file, stdin)
	if err != nil {
		fmt.Fprintf(stdout, "error: %v\n", err)
		return exitUsage
	}
	var listing domain.Listing
	if err := json.Unmarshal(raw, &listing); err != nil {
		fmt.Fprintf(stdout, "error: listing is not valid JSON: %v\n", err)
		return exitUsage
	}
	set, err := loadRules(*rulesPath)
	if err != nil {
		fmt.Fprintf(stdout, "error: %v\n", err)
		return exitRuleSet
	}
	compiled, err := rules.Compile(set, rules.Deps{})
	if err != nil {
		fmt.Fprintf(stdout, "error: %v\n", err)
		return exitRuleSet
	}
	started := time.Now()
	results := detect.Evaluate(context.Background(), compiled.Detectors, listing)
	assessment := risk.Assess(results, compiled, time.Since(started))
	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(assessment); err != nil {
			fmt.Fprintf(stdout, "error: %v\n", err)
			return exitUsage
		}
		return exitOK
	}
	renderText(stdout, assessment)
	return exitOK
}

func readListing(data, file string, stdin io.Reader) ([]byte, error) {
	switch {
	case data != "" && file != "":
		return nil, fmt.Errorf("pass exactly one of --data or --file")
	case data != "":
		return []byte(data), nil
	case file == "-":
		return io.ReadAll(stdin)
	case file != "":
		return os.ReadFile(file)
	default:
		return nil, fmt.Errorf("pass a listing with --data or --file")
	}
}
func loadRules(path string) (rules.RuleSet, error) {
	if path == "" {
		return rules.DefaultRuleSet()
	}
	return rules.Load(path)
}
func renderText(w io.Writer, a risk.Assessment) {
	fmt.Fprintf(w, "Risk %.2f  %s", a.RiskScore, strings.ToUpper(a.RiskBand))
	if a.HardFlagged {
		fmt.Fprint(w, "  (hard flag)")
	}
	fmt.Fprintf(w, "\n%d of %d checks ran in %dms\n", a.Coverage.Ran, a.Coverage.Enabled, a.AnalysisTimeMS)
	section(w, "HIGH RISK", a.HighRisk)
	section(w, "POTENTIALLY RISKY", a.PotentiallyRisky)
	section(w, "POSITIVE SIGNALS", a.PositiveSignals)
	if len(a.PassedChecks) > 0 {
		fmt.Fprintf(w, "\nPASSED (%d)  %s\n", len(a.PassedChecks), strings.Join(a.PassedChecks, ", "))
	}
	if len(a.NotEvaluated) > 0 {
		fmt.Fprintf(w, "\nNOT EVALUATED (%d)\n", len(a.NotEvaluated))
		for _, entry := range a.NotEvaluated {
			fmt.Fprintf(w, "  %-32s %s\n", entry.Rule, entry.Reason)
		}
	}
}
func section(w io.Writer, heading string, findings []risk.Finding) {
	if len(findings) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", heading)
	for _, finding := range findings {
		fmt.Fprintf(w, "  • %s\n", finding.Label)
		if finding.Detail != "" {
			fmt.Fprintf(w, "      %s\n", finding.Detail)
		}
	}
}
func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o600) }
