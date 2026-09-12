// Package marketdata exposes bundled, versioned market-rent benchmarks.
//
// The MVP deliberately contains only HUD Small Area Fair Market Rents for the
// first supported Craigslist markets. It is an offline comparison benchmark,
// not a listing valuation service or a fraud data source.
package marketdata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed fy2027_selected.json
var hudSAFMRJSON []byte

// Source identifies the public dataset behind a Benchmark.
type Source struct {
	Agency         string   `json:"agency"`
	Dataset        string   `json:"dataset"`
	FiscalYear     int      `json:"fiscal_year"`
	EffectiveDate  string   `json:"effective_date"`
	SourceURL      string   `json:"source_url"`
	SourceSHA256   string   `json:"source_sha256"`
	Definition     string   `json:"definition"`
	SupportedAreas []string `json:"supported_areas"`
}

// Benchmark is one ZIP and bedroom-count comparison point. GrossRent is a
// monthly HUD 40th-percentile gross-rent estimate, not a median asking rent.
type Benchmark struct {
	ZIPCode   string
	Areas     []string
	Bedrooms  int
	GrossRent int
	Source    Source
}

type asset struct {
	SchemaVersion int                  `json:"schema_version"`
	Source        Source               `json:"source"`
	ZIPCodes      map[string]zipRecord `json:"zip_codes"`
}

type zipRecord struct {
	Areas []string `json:"areas"`
	Rents []int    `json:"rents"`
}

var (
	loadOnce sync.Once
	loaded   asset
)

// Lookup returns the official bundled HUD benchmark for a five-digit ZIP code
// and a bedroom count from zero (studio) through four. Unsupported ZIPs and
// unavailable bedroom counts intentionally return false rather than falling
// back to a larger geography.
func Lookup(zipCode string, bedrooms int) (Benchmark, bool) {
	if bedrooms < 0 || bedrooms > 4 {
		return Benchmark{}, false
	}
	data := dataset()
	zipCode = strings.TrimSpace(zipCode)
	record, ok := data.ZIPCodes[zipCode]
	if !ok || len(record.Rents) != 5 || record.Rents[bedrooms] <= 0 {
		return Benchmark{}, false
	}
	return Benchmark{
		ZIPCode:   zipCode,
		Areas:     append([]string(nil), record.Areas...),
		Bedrooms:  bedrooms,
		GrossRent: record.Rents[bedrooms],
		Source: Source{
			Agency:         data.Source.Agency,
			Dataset:        data.Source.Dataset,
			FiscalYear:     data.Source.FiscalYear,
			EffectiveDate:  data.Source.EffectiveDate,
			SourceURL:      data.Source.SourceURL,
			SourceSHA256:   data.Source.SourceSHA256,
			Definition:     data.Source.Definition,
			SupportedAreas: append([]string(nil), data.Source.SupportedAreas...),
		},
	}, true
}

func dataset() asset {
	loadOnce.Do(func() {
		if err := json.Unmarshal(hudSAFMRJSON, &loaded); err != nil {
			panic("marketdata: invalid embedded HUD SAFMR data: " + err.Error())
		}
		if err := loaded.validate(); err != nil {
			panic("marketdata: invalid embedded HUD SAFMR data: " + err.Error())
		}
	})
	return loaded
}

func (data asset) validate() error {
	if data.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema version %d", data.SchemaVersion)
	}
	if data.Source.FiscalYear != 2027 || strings.TrimSpace(data.Source.SourceURL) == "" {
		return fmt.Errorf("missing FY 2027 source metadata")
	}
	if len(data.ZIPCodes) == 0 {
		return fmt.Errorf("no ZIP-code benchmarks")
	}
	for zipCode, record := range data.ZIPCodes {
		if len(zipCode) != 5 || strings.Trim(zipCode, "0123456789") != "" {
			return fmt.Errorf("invalid ZIP code %q", zipCode)
		}
		if len(record.Areas) == 0 || len(record.Rents) != 5 {
			return fmt.Errorf("invalid benchmark for ZIP code %s", zipCode)
		}
		for _, rent := range record.Rents {
			if rent <= 0 {
				return fmt.Errorf("invalid rent for ZIP code %s", zipCode)
			}
		}
	}
	return nil
}
