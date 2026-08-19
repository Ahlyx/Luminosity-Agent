package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Findings is the top-level JSON output written to disk.
type Findings struct {
	GeneratedAt   string     `json:"generated_at"`
	CSVPath       string     `json:"csv_path"`
	Focus         []string   `json:"focus,omitempty"`
	Context       string     `json:"context,omitempty"`
	Schema        *Schema    `json:"schema"`
	Overview      [][]string `json:"overview,omitempty"`
	TopRecipients [][]string `json:"top_recipients,omitempty"`
	Concentration [][]string `json:"concentration,omitempty"`
	Anomalies     [][]string `json:"anomalies,omitempty"`
	Temporal      [][]string `json:"temporal,omitempty"`
	FocusResults  [][]string `json:"focus_results,omitempty"`
	Flags         []string   `json:"flags,omitempty"`
}

// StatsInput carries pre-computed statistics needed for flag generation.
type StatsInput struct {
	ConcentrationTop3Pct float64
	AnomalyCount         int
	HurstValue           float64
	PerTermCounts        map[string]int
	PerTermTotals        map[string]float64
}

// buildFindings assembles the Findings struct and auto-generates advisory flags.
func buildFindings(
	csvPath, contextStr string,
	focus []string,
	schema *Schema,
	overview, topRecipients, concentration, anomalies, temporal, focusResults [][]string,
	stats StatsInput,
) Findings {
	f := Findings{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		CSVPath:       csvPath,
		Focus:         focus,
		Context:       contextStr,
		Schema:        schema,
		Overview:      overview,
		TopRecipients: topRecipients,
		Concentration: concentration,
		Anomalies:     anomalies,
		Temporal:      temporal,
		FocusResults:  focusResults,
	}
	f.Flags = generateFlags(stats, focus)
	return f
}

// generateFlags produces advisory strings based on pre-computed stats.
func generateFlags(stats StatsInput, focus []string) []string {
	var flags []string

	if stats.ConcentrationTop3Pct > 50 {
		flags = append(flags, fmt.Sprintf(
			"High concentration: top 3 recipients hold %.1f%% of total spend",
			stats.ConcentrationTop3Pct,
		))
	}

	if stats.AnomalyCount > 0 {
		flags = append(flags, fmt.Sprintf(
			"%d awards exceed 3 standard deviations above mean",
			stats.AnomalyCount,
		))
	}

	if stats.HurstValue > 0.6 {
		flags = append(flags, "Strong upward trend detected")
	}

	for _, term := range focus {
		count := stats.PerTermCounts[term]
		if count == 0 {
			continue
		}
		total := stats.PerTermTotals[term]
		flags = append(flags, fmt.Sprintf(
			"Focus term %q found in %d rows totaling $%.2f",
			term, count, total,
		))
	}

	return flags
}

// writeFindings marshals f to indented JSON and writes it to path.
// If path is empty the default ~/.luminosity/reports/findings_<timestamp>.json is used.
// The parent directory is created if it does not exist.
func writeFindings(f Findings, path string) error {
	if path == "" {
		path = defaultOutPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// defaultOutPath returns ~/.luminosity/reports/findings_<timestamp>.json.
func defaultOutPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	ts := time.Now().Format("20060102_150405")
	return filepath.Join(home, ".luminosity", "reports", fmt.Sprintf("findings_%s.json", ts))
}

// --- helpers used by main to pre-compute StatsInput fields ---

// dataRowCount returns the number of non-header rows in a query result.
// The first row is treated as a header if its first cell is non-numeric.
func dataRowCount(rows [][]string) int {
	if len(rows) == 0 {
		return 0
	}
	start := 0
	if len(rows[0]) > 0 {
		if _, err := strconv.ParseFloat(strings.TrimSpace(rows[0][0]), 64); err != nil {
			start = 1 // first row is a header
		}
	}
	return len(rows) - start
}

// top3PctFromConcentration parses the pct_share column (index 2) from
// concentration results and sums the first three data rows.
func top3PctFromConcentration(rows [][]string) float64 {
	var total float64
	count := 0
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
		if err != nil {
			continue // header or unparseable
		}
		total += v
		count++
		if count >= 3 {
			break
		}
	}
	return total
}

// extractFloatCol parses colIdx from each row, skipping rows where that cell
// is non-numeric. Used to pull a time series out of temporal query results.
func extractFloatCol(rows [][]string, colIdx int) []float64 {
	var out []float64
	for _, row := range rows {
		if colIdx >= len(row) {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(row[colIdx]), 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// countAndSum returns the number of data rows and the sum of the named money
// column from a query result set (first row assumed header when non-numeric).
func countAndSum(rows [][]string, moneyColName string) (count int, total float64) {
	if len(rows) == 0 {
		return
	}
	// Locate money column index in header.
	moneyIdx := -1
	if moneyColName != "" {
		for i, h := range rows[0] {
			if strings.EqualFold(h, moneyColName) {
				moneyIdx = i
				break
			}
		}
	}
	start := 0
	if len(rows[0]) > 0 {
		if _, err := strconv.ParseFloat(strings.TrimSpace(rows[0][0]), 64); err != nil {
			start = 1
		}
	}
	for _, row := range rows[start:] {
		count++
		if moneyIdx >= 0 && moneyIdx < len(row) {
			if v, err := strconv.ParseFloat(strings.TrimSpace(row[moneyIdx]), 64); err == nil {
				total += v
			}
		}
	}
	return
}
