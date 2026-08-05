// Package llm talks to OpenAI-compatible Chat Completions APIs.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal OpenAI-compatible chat client.
type Client struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

// NewClient builds a client. baseURL should include /v1 (no trailing slash required).
func NewClient(apiKey, baseURL, model string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

// Role constants.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is an assistant tool invocation.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds name + JSON arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef is an OpenAI tools[] entry.
type ToolDef struct {
	Type     string             `json:"type"`
	Function ToolDefFunction    `json:"function"`
}

// ToolDefFunction describes a function tool.
type ToolDefFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatRequest is the completions payload.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []ToolDef `json:"tools,omitempty"`
	ToolChoice  any       `json:"tool_choice,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// ChatResponse is the API response.
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

// Usage token counts.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Chat performs one non-streaming chat completion.
func (c *Client) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("llm: missing API key")
	}
	reqBody := ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: 0.2,
	}
	if len(tools) > 0 {
		reqBody.ToolChoice = "auto"
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	url := c.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	var out ChatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("llm: decode: %w\nbody: %s", err, truncate(string(body), 400))
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, fmt.Errorf("llm: %s", out.Error.Message)
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("llm: HTTP %d: %s", res.StatusCode, truncate(string(body), 400))
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("llm: empty choices")
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
