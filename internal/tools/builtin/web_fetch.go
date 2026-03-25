package builtin

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ahlyx/luminosity-agent/internal/tools"
)

type WebFetchTool struct{}

func (t WebFetchTool) Name() string        { return "web_fetch" }
func (t WebFetchTool) Description() string { return "Fetches a URL and returns a plain-text excerpt." }
func (t WebFetchTool) Schema() string {
	return "<tool>web_fetch</tool>\n<url>https://example.com</url>"
}

func (t WebFetchTool) Execute(params map[string]string) (string, error) {
	url := strings.TrimSpace(params["url"])
	if url == "" {
		return "missing parameter: url", nil
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50_000))
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	text := stripHTML(string(body))
	return tools.Truncate(text, 2000), nil
}

var tagRegex = regexp.MustCompile(`<[^>]*>`)
var wsRegex = regexp.MustCompile(`\s+`)

var htmlEntities = strings.NewReplacer(
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
	"&#39;", "'",
)

func stripHTML(input string) string {
	noTags := tagRegex.ReplaceAllString(input, " ")
	collapsed := strings.TrimSpace(wsRegex.ReplaceAllString(noTags, " "))
	return htmlEntities.Replace(collapsed)
}
