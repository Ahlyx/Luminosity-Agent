package builtin

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type ReportStoreTool struct {
	DBPath string
}

func (t *ReportStoreTool) Name() string        { return "report_store" }
func (t *ReportStoreTool) Description() string {
	return "Save an enriched data report with headline and full summary to persistent storage."
}
func (t *ReportStoreTool) Schema() string {
	return "<tool>report_store</tool>\n<path>report-name-slug</path>\n<content>HEADLINE: one line headline\nSUMMARY: full analysis text</content>"
}

func (t *ReportStoreTool) Execute(params map[string]string) (string, error) {
	name := strings.TrimSpace(params["path"])
	if name == "" {
		return "missing parameter: path", nil
	}
	content := strings.TrimSpace(params["content"])
	if content == "" {
		return "missing parameter: content", nil
	}

	headline, summary := parseReportContent(content)

	db, err := t.openDB()
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	defer db.Close()

	res, err := db.Exec(
		`INSERT INTO reports (name, headline, summary) VALUES (?, ?, ?)`,
		name, headline, summary,
	)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	id, _ := res.LastInsertId()

	return fmt.Sprintf("Report '%s' saved (id=%d). Headline: %s", name, id, headline), nil
}

func (t *ReportStoreTool) openDB() (*sql.DB, error) {
	dir := filepath.Dir(t.DBPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
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

// parseReportContent extracts HEADLINE: and SUMMARY: from content.
func parseReportContent(content string) (headline, summary string) {
	lines := strings.Split(content, "\n")
	var summaryLines []string
	inSummary := false

	for _, line := range lines {
		if strings.HasPrefix(line, "HEADLINE:") {
			headline = strings.TrimSpace(strings.TrimPrefix(line, "HEADLINE:"))
			inSummary = false
		} else if strings.HasPrefix(line, "SUMMARY:") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
			if rest != "" {
				summaryLines = append(summaryLines, rest)
			}
			inSummary = true
		} else if inSummary {
			summaryLines = append(summaryLines, line)
		}
	}

	summary = strings.Join(summaryLines, "\n")

	// Fallbacks
	if headline == "" {
		// Use first sentence of content
		idx := strings.IndexAny(content, ".!?\n")
		if idx != -1 {
			headline = strings.TrimSpace(content[:idx+1])
		} else {
			headline = strings.TrimSpace(content)
		}
	}
	if summary == "" {
		summary = content
	}

	return headline, summary
}
