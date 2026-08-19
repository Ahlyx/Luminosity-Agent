package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	csvFlag := flag.String("csv", "", "path to CSV file (required)")
	focusFlag := flag.String("focus", "", "comma-separated focus terms (optional)")
	contextFlag := flag.String("context", "", "free-text context description (optional)")
	outFlag := flag.String("out", "", "output JSON path (default: ~/.luminosity/reports/findings_<timestamp>.json)")
	flag.Parse()

	if *csvFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --csv is required")
		flag.Usage()
		os.Exit(1)
	}

	var focusTerms []string
	if *focusFlag != "" {
		for t := range strings.SplitSeq(*focusFlag, ",") {
			if t = strings.TrimSpace(t); t != "" {
				focusTerms = append(focusTerms, t)
			}
		}
	}

	step := 0
	next := func(label string) {
		step++
		fmt.Printf("[%d] %s\n", step, label)
	}

	next(fmt.Sprintf("Detecting schema for %s ...", *csvFlag))
	schema, err := detectSchema(*csvFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("    %d columns, %d rows  money=%q  date=%q  recipient=%q\n",
		len(schema.Columns), schema.RowCount,
		schema.MoneyCol, schema.DateCol, schema.RecipientCol)

	next("Running overview query ...")
	overview, err := queryOverview(*schema, *csvFlag)
	if err != nil {
		fmt.Println("    warning:", err)
	}

	next("Running top recipients ...")
	topRecipients, err := queryTopRecipients(*schema, *csvFlag)
	if err != nil {
		fmt.Println("    warning:", err)
	}

	next("Running concentration analysis ...")
	concentration, err := queryConcentration(*schema, *csvFlag)
	if err != nil {
		fmt.Println("    warning:", err)
	}

	next("Running anomaly detection ...")
	anomalies, err := queryAnomalies(*schema, *csvFlag)
	if err != nil {
		fmt.Println("    warning:", err)
	}

	next("Running temporal analysis ...")
	temporal, err := queryTemporal(*schema, *csvFlag)
	if err != nil {
		fmt.Println("    warning:", err)
	}

	// --- pre-compute stats for flag generation ---

	anomalyCount := dataRowCount(anomalies)
	top3Pct := top3PctFromConcentration(concentration)

	// Temporal amount column (index 1 = total_amount from queryTemporal).
	temporalAmounts := extractFloatCol(temporal, 1)
	hurstVal := hurstExponent(temporalAmounts)

	perTermCounts := map[string]int{}
	perTermTotals := map[string]float64{}
	var focusResults [][]string

	if len(focusTerms) > 0 {
		next(fmt.Sprintf("Running focus query for %v ...", focusTerms))
		focusResults, err = queryFocus(*schema, *csvFlag, focusTerms)
		if err != nil {
			fmt.Println("    warning:", err)
		}
		// Per-term breakdown for flags.
		for _, term := range focusTerms {
			rows, qErr := queryFocus(*schema, *csvFlag, []string{term})
			if qErr == nil {
				c, t := countAndSum(rows, schema.MoneyCol)
				perTermCounts[term] = c
				perTermTotals[term] = t
			}
		}
	}

	stats := StatsInput{
		ConcentrationTop3Pct: top3Pct,
		AnomalyCount:         anomalyCount,
		HurstValue:           hurstVal,
		PerTermCounts:        perTermCounts,
		PerTermTotals:        perTermTotals,
	}

	f := buildFindings(
		*csvFlag, *contextFlag, focusTerms, schema,
		overview, topRecipients, concentration, anomalies, temporal, focusResults,
		stats,
	)

	outPath := *outFlag
	next(fmt.Sprintf("Writing findings to %s ...", func() string {
		if outPath == "" {
			return defaultOutPath()
		}
		return outPath
	}()))

	if err := writeFindings(f, outPath); err != nil {
		fmt.Fprintln(os.Stderr, "error writing findings:", err)
		os.Exit(1)
	}

	fmt.Printf("\nDone. %d rows analysed. Findings at: %s\n",
		schema.RowCount, f.CSVPath)
}
