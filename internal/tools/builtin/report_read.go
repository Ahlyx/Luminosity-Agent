package builtin

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ahlyx/luminosity-agent/internal/tools"
	_ "modernc.org/sqlite"
)

type ReportReadTool struct {
	DBPath string
}

func (t *ReportReadTool) Name() string        { return "report_read" }
func (t *ReportReadTool) Description() string {
	return "Read a saved report by name, or list all reports."
}
func (t *ReportReadTool) Schema() string {
	return "<tool>report_read</tool>\n<path>report-name-or-list</path>"
}

func (t *ReportReadTool) Execute(params map[string]string) (string, error) {
	path := strings.TrimSpace(params["path"])
	if path == "" {
		return "missing parameter: path", nil
	}

	db, err := t.openReadDB()
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	defer db.Close()

	if path == "list" {
		return t.listReports(db)
	}
	return t.readReport(db, path)
}

func (t *ReportReadTool) openReadDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", t.DBPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		headline TEXT,
		summary TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (t *ReportReadTool) listReports(db *sql.DB) (string, error) {
	rows, err := db.Query(`SELECT id, name, created_at FROM reports ORDER BY id`)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-4s  %-24s  %s\n", "ID", "NAME", "CREATED"))

	count := 0
	for rows.Next() {
		var id int
		var name, createdAt string
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("%-4d  %-24s  %s\n", id, name, createdAt))
		count++
	}
	if count == 0 {
		return "No reports found. Use report_store to save a report.", nil
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func (t *ReportReadTool) readReport(db *sql.DB, name string) (string, error) {
	var id int
	var headline, summary, createdAt string
	err := db.QueryRow(
		`SELECT id, headline, summary, created_at FROM reports WHERE name = ? ORDER BY id DESC LIMIT 1`,
		name,
	).Scan(&id, &headline, &summary, &createdAt)
	if err == sql.ErrNoRows {
		return fmt.Sprintf("No report found with name '%s'. Use report_read with path=list to see all reports.", name), nil
	}
	if err != nil {
		return "Error: " + err.Error(), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s ===\n", name))
	sb.WriteString(fmt.Sprintf("Created: %s\n\n", createdAt))
	sb.WriteString(fmt.Sprintf("HEADLINE: %s\n\n", headline))
	sb.WriteString("SUMMARY:\n")
	sb.WriteString(tools.Truncate(summary, 3000))

	return sb.String(), nil
}
