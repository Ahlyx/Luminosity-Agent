package builtin
 
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
 
	"github.com/ahlyx/luminosity-agent/internal/tools"
)
 
// WebSearchTool performs web searches using Tavily as primary and
// Brave Search as fallback if Tavily fails or returns no results.
type WebSearchTool struct {
	TavilyKey string
	BraveKey  string
}
 
func (t WebSearchTool) Name() string { return "web_search" }
func (t WebSearchTool) Description() string {
	return "Searches the web and returns summarized results. Use this before web_fetch."
}
func (t WebSearchTool) Schema() string {
	return "<tool>web_search</tool>\n<query>OT ICS security research 2024</query>"
}
 
func (t WebSearchTool) Execute(params map[string]string) (string, error) {
	query := strings.TrimSpace(params["query"])
	if query == "" {
		return "missing parameter: query", nil
	}
 
	// Try Tavily first
	if t.TavilyKey != "" {
		result, err := t.tavilySearch(query)
		if err == nil && result != "" {
			return result, nil
		}
		// Log fallback reason but don't surface error to Lumi
		_ = err
	}
 
	// Fallback to Brave
	if t.BraveKey != "" {
		result, err := t.braveSearch(query)
		if err == nil && result != "" {
			return result, nil
		}
		return "Search failed: " + err.Error(), nil
	}
 
	return "No search API keys configured.", nil
}
 
// ── Tavily ────────────────────────────────────────────────────────────────────
 
type tavilyRequest struct {
	APIKey          string   `json:"api_key"`
	Query           string   `json:"query"`
	SearchDepth     string   `json:"search_depth"`
	IncludeAnswer   bool     `json:"include_answer"`
	MaxResults      int      `json:"max_results"`
	IncludeDomains  []string `json:"include_domains,omitempty"`
	ExcludeDomains  []string `json:"exclude_domains,omitempty"`
}
 
type tavilyResponse struct {
	Answer  string `json:"answer"`
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
}
 
func (t WebSearchTool) tavilySearch(query string) (string, error) {
	payload := tavilyRequest{
		APIKey:        t.TavilyKey,
		Query:         query,
		SearchDepth:   "basic",
		IncludeAnswer: true,
		MaxResults:    5,
	}
 
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
 
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(
		"https://api.tavily.com/search",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("tavily request failed: %w", err)
	}
	defer resp.Body.Close()
 
	if resp.StatusCode == 429 {
		return "", fmt.Errorf("tavily rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return "", fmt.Errorf("tavily returned %d: %s", resp.StatusCode, string(b))
	}
 
	var result tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("tavily decode error: %w", err)
	}
 
	return formatTavilyResults(result), nil
}
 
func formatTavilyResults(r tavilyResponse) string {
	var sb strings.Builder
 
	// Include the AI-generated answer if present
	if r.Answer != "" {
		sb.WriteString("Summary: ")
		sb.WriteString(r.Answer)
		sb.WriteString("\n\n")
	}
 
	for i, res := range r.Results {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, res.Title))
		sb.WriteString(fmt.Sprintf("    %s\n", res.URL))
		if res.Content != "" {
			content := res.Content
			if len(content) > 200 {
				content = content[:200] + "…"
			}
			sb.WriteString(fmt.Sprintf("    %s\n", content))
		}
		sb.WriteString("\n")
	}
 
	out := strings.TrimSpace(sb.String())
	return tools.Truncate(out, 2000)
}
 
// ── Brave Search ──────────────────────────────────────────────────────────────
 
type braveSearchResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}
 
func (t WebSearchTool) braveSearch(query string) (string, error) {
	endpoint := "https://api.search.brave.com/res/v1/web/search?q=" +
		url.QueryEscape(query) + "&count=5&text_decorations=false"
 
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Subscription-Token", t.BraveKey)
 
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("brave request failed: %w", err)
	}
	defer resp.Body.Close()
 
	if resp.StatusCode == 429 {
		return "", fmt.Errorf("brave rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return "", fmt.Errorf("brave returned %d: %s", resp.StatusCode, string(b))
	}
 
	var result braveSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("brave decode error: %w", err)
	}
 
	return formatBraveResults(result), nil
}
 
func formatBraveResults(r braveSearchResponse) string {
	if len(r.Web.Results) == 0 {
		return "No results found."
	}
 
	var sb strings.Builder
	for i, res := range r.Web.Results {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, res.Title))
		sb.WriteString(fmt.Sprintf("    %s\n", res.URL))
		if res.Description != "" {
			desc := res.Description
			if len(desc) > 200 {
				desc = desc[:200] + "…"
			}
			sb.WriteString(fmt.Sprintf("    %s\n", desc))
		}
		sb.WriteString("\n")
	}
 
	out := strings.TrimSpace(sb.String())
	return tools.Truncate(out, 2000)
}