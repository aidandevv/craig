package marketdata

import "testing"

func TestLookupReturnsBundledFY2027HUDSAFMR(t *testing.T) {
	benchmark, ok := Lookup("94103", 1)
	if !ok {
		t.Fatal("94103 1BR benchmark is unavailable")
	}
	if benchmark.GrossRent != 3190 || benchmark.Bedrooms != 1 || benchmark.Source.FiscalYear != 2027 {
		t.Errorf("benchmark = %+v", benchmark)
	}
	if benchmark.Source.EffectiveDate != "2026-10-01" || len(benchmark.Areas) != 1 || benchmark.Areas[0] != "San Francisco, CA HUD Metro FMR Area" {
		t.Errorf("source/area = %+v", benchmark)
	}
}

func TestLookupKeepsIdenticalCrossAreaZIPBenchmarks(t *testing.T) {
	benchmark, ok := Lookup("94020", 2)
	if !ok || benchmark.GrossRent != 4100 {
		t.Fatalf("94020 2BR benchmark = %+v, found=%v", benchmark, ok)
	}
	if len(benchmark.Areas) != 2 {
		t.Errorf("areas = %v, want both HUD areas", benchmark.Areas)
	}
}

func TestLookupRejectsUnsupportedInputsWithoutFallback(t *testing.T) {
	for _, test := range []struct {
		zipCode  string
		bedrooms int
	}{
		{"99999", 1},
		{"94103", -1},
		{"94103", 5},
		{"94103-1234", 1},
	} {
		if got, ok := Lookup(test.zipCode, test.bedrooms); ok {
			t.Errorf("Lookup(%q, %d) = %+v, want unavailable", test.zipCode, test.bedrooms, got)
		}
	}
}
