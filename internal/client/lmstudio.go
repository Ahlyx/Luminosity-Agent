package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LMStudioClient struct {
	baseURL    string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

func New(baseURL, model string, timeoutSeconds int) *LMStudioClient {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	return &LMStudioClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		timeout: time.Duration(timeoutSeconds) * time.Second,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Stop        []string  `json:"stop"`
}

type streamChunk struct {
    Choices []struct {
        Delta struct {
            Content          string `json:"content"`
            ReasoningContent string `json:"reasoning_content"`
        } `json:"delta"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
}

func (c *LMStudioClient) StreamChat(messages []Message, maxTokens int, onToken func(string)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	body := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Stream:      true,
		Temperature: 0.7,
		MaxTokens:   maxTokens,
		Stop:        []string{},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("LM Studio response timed out. Try again.")
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return "", fmt.Errorf("LM Studio response timed out. Try again.")
		}
		if strings.Contains(strings.ToLower(err.Error()), "connection refused") {
			return "", fmt.Errorf("Cannot reach LM Studio at %s - is it running?", c.baseURL)
		}
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 3000))
		return "", fmt.Errorf("LM Studio returned %s: %s", resp.Status, string(b))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		tok := chunk.Choices[0].Delta.Content
		if tok == "" {
			continue
		}
		full.WriteString(tok)
		if onToken != nil {
			onToken(tok)
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return full.String(), fmt.Errorf("LM Studio response timed out. Try again.")
		}
		return full.String(), err
	}

	return full.String(), nil
}
