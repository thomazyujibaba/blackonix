package ports

import "context"

// ToolDefinition representa uma tool disponível para a LLM no formato Function Calling.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ToolCall representa uma chamada de tool retornada pela LLM.
type ToolCall struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
}

// LLMMessage representa uma mensagem no formato da LLM.
type LLMMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// LLMResponse representa a resposta da LLM.
type LLMResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// LLMClient define a interface para comunicação com provedores de LLM.
type LLMClient interface {
	ChatCompletion(ctx context.Context, messages []LLMMessage, tools []ToolDefinition) (*LLMResponse, error)
}
