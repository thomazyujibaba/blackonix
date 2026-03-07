package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"blackonix/internal/ports"
)

const (
	openaiURL         = "https://api.openai.com/v1/chat/completions"
	openaiHTTPTimeout = 45 * time.Second
)

type openaiClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

func NewOpenAIClient(apiKey, model string) ports.LLMClient {
	return &openaiClient{
		httpClient: &http.Client{Timeout: openaiHTTPTimeout},
		apiKey:     apiKey,
		model:      model,
	}
}

func (c *openaiClient) ChatCompletion(ctx context.Context, messages []ports.LLMMessage, tools []ports.ToolDefinition) (*ports.LLMResponse, error) {
	body := map[string]interface{}{
		"model":    c.model,
		"messages": convertMessages(messages),
	}

	if len(tools) > 0 {
		body["tools"] = convertTools(tools)
		body["tool_choice"] = "auto"
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openai API returned status %d", resp.StatusCode)
	}

	var result openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := result.Choices[0].Message

	llmResp := &ports.LLMResponse{
		Content: choice.Content,
	}

	for _, tc := range choice.ToolCalls {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
			params = map[string]interface{}{"raw": tc.Function.Arguments}
		}
		llmResp.ToolCalls = append(llmResp.ToolCalls, ports.ToolCall{
			ID:         tc.ID,
			Name:       tc.Function.Name,
			Parameters: params,
		})
	}

	return llmResp, nil
}

// Tipos internos para deserializar a resposta da OpenAI.
type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
}

type openaiChoice struct {
	Message openaiMessage `json:"message"`
}

type openaiMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func convertMessages(msgs []ports.LLMMessage) []map[string]interface{} {
	var result []map[string]interface{}
	for _, m := range msgs {
		msg := map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		result = append(result, msg)
	}
	return result
}

func convertTools(tools []ports.ToolDefinition) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}
	return result
}
