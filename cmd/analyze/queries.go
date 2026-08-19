package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var fromDataRe = regexp.MustCompile(`(?i)\bFROM\s+data\b`)

// runQuery executes sql via "duckdb -csv -c", replacing "FROM data" with
// FROM read_csv_auto('/csvPath'). Times out after 120 seconds.
// Returns stderr content in the error when exit code is non-zero.
func runQuery(csvPath, sql string) ([][]string, error) {
	fwdPath := strings.ReplaceAll(csvPath, `\`, `/`)
	replacement := fmt.Sprintf("FROM read_csv_auto('%s')", fwdPath)
	sql = fromDataRe.ReplaceAllString(sql, replacement)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "duckdb", "-csv", "-c", sql)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}

	r := csv.NewReader(strings.NewReader(stdout.String()))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// queryOverview returns total rows, sum of money column, and date range.
func queryOverview(s Schema, csvPath string) ([][]string, error) {
	selects := []string{"COUNT(*) AS row_count"}
	if s.MoneyCol != "" {
		selects = append(selects, fmt.Sprintf(`SUM("%s") AS total_amount`, s.MoneyCol))
		selects = append(selects, fmt.Sprintf(`AVG("%s") AS avg_amount`, s.MoneyCol))
		selects = append(selects, fmt.Sprintf(`MIN("%s") AS min_amount`, s.MoneyCol))
		selects = append(selects, fmt.Sprintf(`MAX("%s") AS max_amount`, s.MoneyCol))
	}
	if s.DateCol != "" {
		selects = append(selects, fmt.Sprintf(`MIN(TRY_CAST("%s" AS DATE)) AS earliest_date`, s.DateCol))
		selects = append(selects, fmt.Sprintf(`MAX(TRY_CAST("%s" AS DATE)) AS latest_date`, s.DateCol))
	}
	if s.RecipientCol != "" {
		selects = append(selects, fmt.Sprintf(`COUNT(DISTINCT "%s") AS unique_recipients`, s.RecipientCol))
	}
	if s.AgencyCol != "" {
		selects = append(selects, fmt.Sprintf(`COUNT(DISTINCT "%s") AS unique_agencies`, s.AgencyCol))
	}
	sql := fmt.Sprintf("SELECT %s FROM data", strings.Join(selects, ", "))
	return runQuery(csvPath, sql)
}

// queryTopRecipients groups by recipient (or agency) and sums the money column.
func queryTopRecipients(s Schema, csvPath string) ([][]string, error) {
	groupCol := s.RecipientCol
	if groupCol == "" {
		groupCol = s.AgencyCol
	}
	if groupCol == "" {
		return nil, fmt.Errorf("no recipient or agency column detected")
	}
	if s.MoneyCol == "" {
		sql := fmt.Sprintf(
			`SELECT "%s", COUNT(*) AS awards FROM data GROUP BY "%s" ORDER BY awards DESC LIMIT 20`,
			groupCol, groupCol,
		)
		return runQuery(csvPath, sql)
	}
	sql := fmt.Sprintf(
		`SELECT "%s", SUM("%s") AS total_amount, COUNT(*) AS awards `+
			`FROM data GROUP BY "%s" ORDER BY total_amount DESC LIMIT 20`,
		groupCol, s.MoneyCol, groupCol,
	)
	return runQuery(csvPath, sql)
}

// queryConcentration returns the top-10 recipients and their percentage share of total spend.
func queryConcentration(s Schema, csvPath string) ([][]string, error) {
	groupCol := s.RecipientCol
	if groupCol == "" {
		groupCol = s.AgencyCol
	}
	if groupCol == "" {
		return nil, fmt.Errorf("no recipient or agency column detected")
	}
	if s.MoneyCol == "" {
		return nil, fmt.Errorf("no money column detected")
	}
	sql := fmt.Sprintf(
		`WITH ranked AS (
  SELECT "%s" AS entity, SUM("%s") AS amount
  FROM data
  GROUP BY "%s"
  ORDER BY amount DESC
  LIMIT 10
),
grand AS (SELECT SUM("%s") AS total FROM data)
SELECT entity, amount, ROUND(100.0 * amount / (SELECT total FROM grand), 4) AS pct_share
FROM ranked
ORDER BY amount DESC`,
		groupCol, s.MoneyCol, groupCol, s.MoneyCol,
	)
	return runQuery(csvPath, sql)
}

// queryAnomalies finds rows where the money column exceeds mean + 3 standard deviations.
func queryAnomalies(s Schema, csvPath string) ([][]string, error) {
	if s.MoneyCol == "" {
		return nil, fmt.Errorf("no money column detected")
	}

	selects := []string{fmt.Sprintf(`"%s"`, s.MoneyCol)}
	if s.RecipientCol != "" {
		selects = append(selects, fmt.Sprintf(`"%s"`, s.RecipientCol))
	}
	if s.DateCol != "" {
		selects = append(selects, fmt.Sprintf(`"%s"`, s.DateCol))
	}
	if s.AgencyCol != "" {
		selects = append(selects, fmt.Sprintf(`"%s"`, s.AgencyCol))
	}

	sql := fmt.Sprintf(
		`WITH stats AS (
  SELECT AVG("%s") AS mu, STDDEV_SAMP("%s") AS sigma FROM data
)
SELECT %s
FROM data, stats
WHERE stats.sigma > 0
  AND "%s" > stats.mu + 3 * stats.sigma
ORDER BY "%s" DESC
LIMIT 50`,
		s.MoneyCol, s.MoneyCol,
		strings.Join(selects, ", "),
		s.MoneyCol, s.MoneyCol,
	)
	return runQuery(csvPath, sql)
}

// queryTemporal groups spend by calendar year.
func queryTemporal(s Schema, csvPath string) ([][]string, error) {
	if s.DateCol == "" {
		return nil, fmt.Errorf("no date column detected")
	}
	if s.MoneyCol == "" {
		sql := fmt.Sprintf(
			`SELECT YEAR(TRY_CAST("%s" AS DATE)) AS yr, COUNT(*) AS awards `+
				`FROM data GROUP BY yr ORDER BY yr`,
			s.DateCol,
		)
		return runQuery(csvPath, sql)
	}
	sql := fmt.Sprintf(
		`SELECT YEAR(TRY_CAST("%s" AS DATE)) AS yr, SUM("%s") AS total_amount, COUNT(*) AS awards `+
			`FROM data GROUP BY yr ORDER BY yr`,
		s.DateCol, s.MoneyCol,
	)
	return runQuery(csvPath, sql)
}

// queryFocus filters rows matching any of the provided terms across text/category columns.
func queryFocus(s Schema, csvPath string, terms []string) ([][]string, error) {
	if len(terms) == 0 {
		return nil, fmt.Errorf("no focus terms provided")
	}

	// Collect searchable columns: kind must be text/category AND the raw DuckDB
	// type must be a string type so we never apply ILIKE to BOOLEAN or numerics.
	var searchCols []string
	seen := map[string]bool{}
	for _, c := range s.Columns {
		if (c.Kind == KindText || c.Kind == KindCategory) && isStringType(strings.ToLower(c.Type)) {
			if !seen[c.Name] {
				searchCols = append(searchCols, c.Name)
				seen[c.Name] = true
			}
		}
	}
	// RecipientCol / AgencyCol are added only when their type is also a string type.
	colTypeMap := make(map[string]string, len(s.Columns))
	for _, c := range s.Columns {
		colTypeMap[c.Name] = c.Type
	}
	for _, name := range []string{s.RecipientCol, s.AgencyCol} {
		if name == "" || seen[name] {
			continue
		}
		if isStringType(strings.ToLower(colTypeMap[name])) {
			searchCols = append(searchCols, name)
			seen[name] = true
		}
	}
	if len(searchCols) == 0 {
		return nil, fmt.Errorf("no searchable text columns found")
	}

	// Build WHERE: (col ILIKE '%term%' OR ...) for each term across all searchable cols.
	var orClauses []string
	for _, term := range terms {
		escaped := strings.ReplaceAll(term, "'", "''")
		for _, col := range searchCols {
			orClauses = append(orClauses, fmt.Sprintf(`"%s" ILIKE '%%%s%%'`, col, escaped))
		}
	}

	seenSel := map[string]bool{}
	var selects []string
	for _, name := range []string{s.RecipientCol, s.AgencyCol, s.DateCol, s.MoneyCol} {
		quoted := fmt.Sprintf(`"%s"`, name)
		if name != "" && !seenSel[quoted] {
			selects = append(selects, quoted)
			seenSel[quoted] = true
		}
	}
	if len(selects) == 0 {
		selects = []string{"*"}
	}

	orderClause := ""
	if s.MoneyCol != "" {
		orderClause = fmt.Sprintf(` ORDER BY "%s" DESC`, s.MoneyCol)
	}

	sql := fmt.Sprintf(
		`SELECT %s FROM data WHERE %s%s LIMIT 100`,
		strings.Join(selects, ", "),
		strings.Join(orClauses, " OR "),
		orderClause,
	)
	return runQuery(csvPath, sql)
}
