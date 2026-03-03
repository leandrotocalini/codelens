package summarizer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	defaultChatCompletionsURL = "https://api.openai.com/v1/chat/completions"
	defaultCodexResponsesURL  = "https://chatgpt.com/backend-api/codex/responses"
)

// Summarizer is the interface for LLM-based summarization.
type Summarizer interface {
	Summarize(ctx context.Context, prompt string) (string, error)
}

// NewSummarizer creates a Codex-only summarizer.
// provider must be empty or "codex".
func NewSummarizer(provider, model, oauthToken string, debug bool, debugOut io.Writer) (Summarizer, error) {
	switch provider {
	case "", "codex":
	default:
		return nil, fmt.Errorf("unsupported provider %q: only codex is available", provider)
	}
	if model == "" {
		model = "codex"
	}
	if oauthToken == "" {
		return nil, errors.New("oauth token is required")
	}

	return &CodexSummarizer{
		Model:              model,
		OAuthToken:         oauthToken,
		ChatCompletionsURL: defaultChatCompletionsURL,
		CodexResponsesURL:  defaultCodexResponsesURL,
		HTTPClient:         http.DefaultClient,
		Debug:              debug,
		DebugWriter:        debugOut,
	}, nil
}

// CodexSummarizer talks to Codex endpoints using OAuth bearer tokens.
type CodexSummarizer struct {
	Model              string
	OAuthToken         string
	ChatCompletionsURL string
	CodexResponsesURL  string
	HTTPClient         *http.Client
	Debug              bool
	DebugWriter        io.Writer
}

type chatCompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []chatMessage     `json:"messages"`
	Tools       []json.RawMessage `json:"tools,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type codexResponsesRequest struct {
	Model        string              `json:"model"`
	Instructions string              `json:"instructions,omitempty"`
	Input        []codexInputMessage `json:"input"`
	Stream       bool                `json:"stream"`
	Store        bool                `json:"store"`
}

type codexInputMessage struct {
	Role    string              `json:"role"`
	Content []codexInputContent `json:"content"`
}

type codexInputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type normalizedResponse struct {
	Text      string
	ToolCalls []map[string]any
	Usage     map[string]any
}

func (c *CodexSummarizer) Summarize(ctx context.Context, prompt string) (string, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	if c.Debug && c.DebugWriter == nil {
		c.DebugWriter = os.Stderr
	}

	req := chatCompletionRequest{
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 1024,
	}

	var lastErr error
	for _, model := range modelFallbackOrder(c.Model) {
		req.Model = model
		text, err := c.callWithFallbackTransport(ctx, req)
		if err == nil {
			return text, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("all model fallbacks failed: %w", lastErr)
}

func (c *CodexSummarizer) callWithFallbackTransport(ctx context.Context, req chatCompletionRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}
	c.debugf("chat.completions request model=%s endpoint=%s payload=%s", req.Model, c.ChatCompletionsURL, sanitizeAndCompact(body))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ChatCompletionsURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.OAuthToken)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	c.debugf("chat.completions response status=%d body=%s", resp.StatusCode, sanitizeAndCompact(respBody))
	if resp.StatusCode == http.StatusUnauthorized && strings.Contains(string(respBody), "Missing scopes: model.request") {
		return c.callCodexResponsesFallback(ctx, req)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, compactErrorMessage(respBody))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", errors.New("empty response from API")
	}
	return result.Choices[0].Message.Content, nil
}

func (c *CodexSummarizer) callCodexResponsesFallback(ctx context.Context, req chatCompletionRequest) (string, error) {
	var instructionsParts []string
	var input []codexInputMessage
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system", "developer":
			instructionsParts = append(instructionsParts, msg.Content)
		default:
			input = append(input, codexInputMessage{
				Role: msg.Role,
				Content: []codexInputContent{
					{
						Type: "input_text",
						Text: msg.Content,
					},
				},
			})
		}
	}
	if len(input) == 0 {
		input = append(input, codexInputMessage{
			Role: "user",
			Content: []codexInputContent{
				{
					Type: "input_text",
					Text: "",
				},
			},
		})
	}

	fallbackReq := codexResponsesRequest{
		Model:        req.Model,
		Instructions: strings.Join(instructionsParts, "\n\n"),
		Input:        input,
		Stream:       true,
		Store:        false,
	}
	if strings.TrimSpace(fallbackReq.Instructions) == "" {
		fallbackReq.Instructions = "You are a precise coding assistant. Follow the user input and return only the requested output."
	}

	body, err := json.Marshal(fallbackReq)
	if err != nil {
		return "", fmt.Errorf("marshaling fallback request: %w", err)
	}
	c.debugf("codex.responses request model=%s endpoint=%s payload=%s", req.Model, c.CodexResponsesURL, sanitizeAndCompact(body))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.CodexResponsesURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating fallback request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.OAuthToken)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("fallback request failed: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	c.debugf("codex.responses response status=%d content-type=%s", resp.StatusCode, contentType)
	if strings.Contains(contentType, "text/event-stream") {
		if resp.StatusCode != http.StatusOK {
			respBody, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return "", fmt.Errorf("reading fallback error response: %w", readErr)
			}
			return "", fmt.Errorf("fallback API error (status %d): %s", resp.StatusCode, compactErrorMessage(respBody))
		}
		normalized, err := parseCodexSSEStream(resp.Body)
		if err != nil {
			return "", err
		}
		if normalized.Text == "" {
			return "", errors.New("empty response from fallback API")
		}
		c.debugf("codex.responses stream normalized_text=%s", sanitizeAndCompact([]byte(normalized.Text)))
		return normalized.Text, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading fallback response: %w", err)
	}
	c.debugf("codex.responses response body=%s", sanitizeAndCompact(respBody))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fallback API error (status %d): %s", resp.StatusCode, compactErrorMessage(respBody))
	}

	normalized, err := parseCodexResponse(resp.Header.Get("Content-Type"), respBody)
	if err != nil {
		return "", err
	}
	if normalized.Text == "" {
		return "", errors.New("empty response from fallback API")
	}
	return normalized.Text, nil
}

func modelFallbackOrder(model string) []string {
	switch model {
	case "codex":
		return []string{"codex", "gpt-5.3-codex", "gpt-5.2-codex"}
	case "gpt-5.3-codex":
		return []string{"gpt-5.3-codex", "gpt-5.2-codex"}
	default:
		return []string{model}
	}
}

func parseCodexResponse(contentType string, body []byte) (normalizedResponse, error) {
	if strings.Contains(contentType, "text/event-stream") || bytes.Contains(body, []byte("data:")) {
		return parseCodexSSE(body)
	}
	return parseCodexJSON(body)
}

func parseCodexJSON(body []byte) (normalizedResponse, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return normalizedResponse{}, fmt.Errorf("parsing fallback JSON: %w", err)
	}
	return normalizeFromMap(raw), nil
}

func parseCodexSSE(body []byte) (normalizedResponse, error) {
	var agg normalizedResponse
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			continue
		}
		current := normalizeFromMap(raw)
		agg.Text += current.Text
		if len(current.ToolCalls) > 0 {
			agg.ToolCalls = append(agg.ToolCalls, current.ToolCalls...)
		}
		if current.Usage != nil {
			agg.Usage = current.Usage
		}
	}
	return agg, nil
}

func parseCodexSSEStream(r io.Reader) (normalizedResponse, error) {
	var agg normalizedResponse
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return agg, nil
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			continue
		}
		current := normalizeFromMap(raw)
		agg.Text += current.Text
		if len(current.ToolCalls) > 0 {
			agg.ToolCalls = append(agg.ToolCalls, current.ToolCalls...)
		}
		if current.Usage != nil {
			agg.Usage = current.Usage
		}
	}
	if err := scanner.Err(); err != nil {
		return normalizedResponse{}, fmt.Errorf("reading fallback SSE stream: %w", err)
	}
	return agg, nil
}

func normalizeFromMap(raw map[string]any) normalizedResponse {
	var out normalizedResponse
	if text, ok := raw["output_text"].(string); ok {
		out.Text = text
	}
	if delta, ok := raw["delta"].(string); ok {
		out.Text += delta
	}
	if usage, ok := raw["usage"].(map[string]any); ok {
		out.Usage = usage
	}
	if response, ok := raw["response"].(map[string]any); ok {
		if usage, ok := response["usage"].(map[string]any); ok {
			out.Usage = usage
		}
		if combined := extractTextFromResponseOutput(response["output"]); combined != "" {
			out.Text += combined
		}
	}
	if output := extractTextFromResponseOutput(raw["output"]); output != "" {
		out.Text += output
	}
	if toolCalls := extractToolCalls(raw); len(toolCalls) > 0 {
		out.ToolCalls = toolCalls
	}
	return out
}

func extractTextFromResponseOutput(v any) string {
	entries, ok := v.([]any)
	if !ok {
		return ""
	}
	var builder strings.Builder
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		content, ok := item["content"].([]any)
		if !ok {
			continue
		}
		for _, chunk := range content {
			chunkMap, ok := chunk.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := chunkMap["text"].(string); ok {
				builder.WriteString(text)
			}
		}
	}
	return builder.String()
}

func extractToolCalls(raw map[string]any) []map[string]any {
	toolCallsRaw, ok := raw["tool_calls"].([]any)
	if !ok {
		return nil
	}
	toolCalls := make([]map[string]any, 0, len(toolCallsRaw))
	for _, v := range toolCallsRaw {
		call, ok := v.(map[string]any)
		if ok {
			toolCalls = append(toolCalls, call)
		}
	}
	return toolCalls
}

func compactErrorMessage(body []byte) string {
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		return msg[:300]
	}
	return msg
}

func sanitizeAndCompact(body []byte) string {
	msg := strings.TrimSpace(string(body))
	msg = strings.ReplaceAll(msg, `"authorization":"`, `"authorization":"[REDACTED]`)
	if len(msg) > 1200 {
		return msg[:1200] + "...(truncated)"
	}
	return msg
}

func (c *CodexSummarizer) debugf(format string, args ...any) {
	if !c.Debug {
		return
	}
	w := c.DebugWriter
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, "[debug] "+format+"\n", args...)
}
