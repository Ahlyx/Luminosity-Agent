package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ColKind categorizes a column's semantic role.
type ColKind string

const (
	KindMoney    ColKind = "money"
	KindDate     ColKind = "date"
	KindCategory ColKind = "category"
	KindID       ColKind = "id"
	KindText     ColKind = "text"
	KindNumeric  ColKind = "numeric"
)

// Column is one field in the detected schema.
type Column struct {
	Name string  `json:"name"`
	Type string  `json:"type"`
	Kind ColKind `json:"kind"`
}

// Schema holds all detected column metadata and best-candidate column names.
type Schema struct {
	Columns      []Column `json:"columns"`
	RowCount     int64    `json:"row_count"`
	MoneyCol     string   `json:"money_col"`
	DateCol      string   `json:"date_col"`
	RecipientCol string   `json:"recipient_col"`
	AgencyCol    string   `json:"agency_col"`
}

// detectSchema runs DuckDB to DESCRIBE and COUNT the CSV file, then classifies columns.
func detectSchema(csvPath string) (*Schema, error) {
	fwdPath := strings.ReplaceAll(csvPath, `\`, `/`)

	descSQL := fmt.Sprintf(
		"SELECT column_name, column_type FROM (DESCRIBE SELECT * FROM read_csv_auto('%s', header=true)) LIMIT 200",
		fwdPath,
	)
	fmt.Fprintf(os.Stderr, "[schema] DESCRIBE SQL: %s\n", descSQL)
	rows, rawOut, rawErr, err := execDuckDB(descSQL)
	if err != nil {
		return nil, fmt.Errorf("describe: %w", err)
	}
	if len(rows) == 0 {
		fmt.Fprintf(os.Stderr, "[schema] DESCRIBE returned 0 rows\n")
		fmt.Fprintf(os.Stderr, "[schema] stdout: %s\n", rawOut)
		fmt.Fprintf(os.Stderr, "[schema] stderr: %s\n", rawErr)
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM read_csv_auto('%s', header=true)", fwdPath)
	countRows, _, _, err := execDuckDB(countSQL)
	if err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}

	var rowCount int64
	// countRows may be [[header],[value]] or just [[value]] depending on DuckDB version.
	for _, row := range countRows {
		if len(row) == 0 {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
		if err == nil {
			rowCount = n
			break
		}
	}

	// Parse DESCRIBE output — first row may be a header.
	var columns []Column
	start := 0
	if len(rows) > 0 && len(rows[0]) > 0 && strings.ToLower(rows[0][0]) == "column_name" {
		start = 1
	}
	for _, row := range rows[start:] {
		if len(row) < 2 || row[0] == "" {
			continue
		}
		name := row[0]
		typ := row[1]
		columns = append(columns, Column{Name: name, Type: typ, Kind: classifyColumn(name, typ)})
	}

	s := &Schema{
		Columns:  columns,
		RowCount: rowCount,
	}
	s.MoneyCol = detectMoneyCol(columns)
	s.DateCol = detectDateCol(columns)
	s.RecipientCol = detectRecipientCol(columns)
	s.AgencyCol = detectAgencyCol(columns)

	return s, nil
}

// execDuckDB runs a raw SQL string via "duckdb -csv -c".
// Returns parsed CSV rows plus the raw stdout and stderr strings for diagnostics.
func execDuckDB(sql string) (rows [][]string, rawOut, rawErr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "duckdb", "-csv", "-c", sql)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		return nil, stdout.String(), stderr.String(),
			fmt.Errorf("%w: %s", runErr, strings.TrimSpace(stderr.String()))
	}

	rawOut = stdout.String()
	rawErr = stderr.String()

	r := csv.NewReader(strings.NewReader(rawOut))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	rows, err = r.ReadAll()
	return
}

// --- Column classification helpers ---

var moneyWords = []string{
	"obligation", "amount", "value", "total", "cost", "price", "dollar", "fund", "award",
}
var dateWords = []string{
	"date", "time", "year", "period", "action", "fiscal", "start", "end",
}
var idWords = []string{
	"duns", "uei", "cage", "naics", "psc", "ein", "fein", "zip",
}
var categoryWords = []string{
	"type", "category", "status", "agency", "department", "program",
	"set_aside", "extent", "competition", "method",
}

func containsAny(s string, words []string) bool {
	low := strings.ToLower(s)
	for _, w := range words {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

func classifyColumn(name, typ string) ColKind {
	lowTyp := strings.ToLower(typ)

	// Date/timestamp types take precedence.
	if strings.HasPrefix(lowTyp, "date") ||
		strings.HasPrefix(lowTyp, "timestamp") ||
		strings.HasPrefix(lowTyp, "time") {
		return KindDate
	}

	isNumeric := strings.HasPrefix(lowTyp, "int") ||
		strings.HasPrefix(lowTyp, "bigint") ||
		strings.HasPrefix(lowTyp, "smallint") ||
		strings.HasPrefix(lowTyp, "tinyint") ||
		strings.HasPrefix(lowTyp, "hugeint") ||
		strings.HasPrefix(lowTyp, "double") ||
		strings.HasPrefix(lowTyp, "float") ||
		strings.HasPrefix(lowTyp, "real") ||
		strings.HasPrefix(lowTyp, "decimal") ||
		strings.HasPrefix(lowTyp, "numeric")

	if isNumeric {
		if containsAny(name, moneyWords) {
			return KindMoney
		}
		if containsAny(name, dateWords) {
			return KindDate
		}
		return KindNumeric
	}

	// Only assign text/category/ID kinds to actual string types.
	// BOOLEAN and any other non-string type that survived the numeric check above
	// gets KindNumeric so it is never passed to ILIKE.
	if !isStringType(lowTyp) {
		return KindNumeric
	}

	// VARCHAR / TEXT: classify by name.
	if containsAny(name, moneyWords) {
		return KindMoney
	}
	if containsAny(name, dateWords) {
		return KindDate
	}
	lowName := strings.ToLower(name)
	if containsAny(name, idWords) ||
		strings.HasSuffix(lowName, "_id") ||
		strings.HasSuffix(lowName, "_code") ||
		strings.HasSuffix(lowName, "_number") {
		return KindID
	}
	if containsAny(name, categoryWords) {
		return KindCategory
	}
	return KindText
}

// isStringType reports whether a lower-cased DuckDB type string represents a
// character/string type that can safely receive an ILIKE predicate.
func isStringType(lowTyp string) bool {
	return strings.Contains(lowTyp, "varchar") ||
		strings.Contains(lowTyp, "text") ||
		strings.Contains(lowTyp, "char") ||
		strings.Contains(lowTyp, "string")
}

// --- Auto-detect best candidate columns ---

func detectMoneyCol(cols []Column) string {
	// Prefer the canonical USASpending obligations field first.
	for _, c := range cols {
		if strings.ToLower(c.Name) == "federal_action_obligation" {
			return c.Name
		}
	}
	for _, c := range cols {
		if strings.Contains(strings.ToLower(c.Name), "obligation") {
			return c.Name
		}
	}
	for _, c := range cols {
		if c.Kind == KindMoney {
			return c.Name
		}
	}
	for _, c := range cols {
		if c.Kind == KindNumeric {
			return c.Name
		}
	}
	return ""
}

func detectDateCol(cols []Column) string {
	for _, c := range cols {
		if strings.ToLower(c.Name) == "action_date" {
			return c.Name
		}
	}
	for _, c := range cols {
		if strings.Contains(strings.ToLower(c.Name), "action") && strings.Contains(strings.ToLower(c.Name), "date") {
			return c.Name
		}
	}
	for _, c := range cols {
		if c.Kind == KindDate {
			return c.Name
		}
	}
	return ""
}

func detectRecipientCol(cols []Column) string {
	for _, c := range cols {
		if strings.ToLower(c.Name) == "recipient_name" {
			return c.Name
		}
	}
	for _, c := range cols {
		low := strings.ToLower(c.Name)
		if strings.Contains(low, "recipient") {
			return c.Name
		}
	}
	for _, c := range cols {
		low := strings.ToLower(c.Name)
		if strings.Contains(low, "vendor") || strings.Contains(low, "awardee") || strings.Contains(low, "contractor") {
			return c.Name
		}
	}
	return ""
}

func detectAgencyCol(cols []Column) string {
	for _, c := range cols {
		if strings.ToLower(c.Name) == "awarding_agency_name" {
			return c.Name
		}
	}
	for _, c := range cols {
		if strings.Contains(strings.ToLower(c.Name), "agency") {
			return c.Name
		}
	}
	for _, c := range cols {
		if c.Kind == KindCategory {
			return c.Name
		}
	}
	return ""
}
