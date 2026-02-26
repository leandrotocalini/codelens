package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Summarizer is the interface for LLM-based summarization.
type Summarizer interface {
	Summarize(ctx context.Context, prompt string) (string, error)
}

// NewSummarizer creates a Summarizer for the given provider.
func NewSummarizer(provider, model, apiKey string) (Summarizer, error) {
	switch provider {
	case "anthropic":
		return &AnthropicSummarizer{Model: model, APIKey: apiKey}, nil
	case "openai":
		return &OpenAISummarizer{
			Model:   model,
			APIKey:  apiKey,
			BaseURL: "https://api.openai.com/v1",
		}, nil
	case "openrouter":
		return &OpenAISummarizer{
			Model:   model,
			APIKey:  apiKey,
			BaseURL: "https://openrouter.ai/api/v1",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// AnthropicSummarizer uses the Anthropic Messages API.
type AnthropicSummarizer struct {
	Model  string
	APIKey string
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *AnthropicSummarizer) Summarize(ctx context.Context, prompt string) (string, error) {
	req := anthropicRequest{
		Model:     a.Model,
		MaxTokens: 1024,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	return a.doRequest(ctx, body)
}

func (a *AnthropicSummarizer) doRequest(ctx context.Context, body []byte) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", a.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
			if resp.StatusCode >= 500 {
				continue
			}
			return "", lastErr
		}

		var result anthropicResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return "", fmt.Errorf("parsing response: %w", err)
		}

		if result.Error != nil {
			return "", fmt.Errorf("API error: %s", result.Error.Message)
		}

		if len(result.Content) == 0 {
			return "", fmt.Errorf("empty response from API")
		}

		return result.Content[0].Text, nil
	}
	return "", fmt.Errorf("after 3 retries: %w", lastErr)
}

// OpenAISummarizer uses the OpenAI-compatible chat completions API.
// Also used for OpenRouter (same API format, different base URL).
type OpenAISummarizer struct {
	Model   string
	APIKey  string
	BaseURL string
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAISummarizer) Summarize(ctx context.Context, prompt string) (string, error) {
	req := openaiRequest{
		Model: o.Model,
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	return o.doRequest(ctx, body)
}

func (o *OpenAISummarizer) doRequest(ctx context.Context, body []byte) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		url := o.BaseURL + "/chat/completions"
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
			if resp.StatusCode >= 500 {
				continue
			}
			return "", lastErr
		}

		var result openaiResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return "", fmt.Errorf("parsing response: %w", err)
		}

		if result.Error != nil {
			return "", fmt.Errorf("API error: %s", result.Error.Message)
		}

		if len(result.Choices) == 0 {
			return "", fmt.Errorf("empty response from API")
		}

		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("after 3 retries: %w", lastErr)
}
