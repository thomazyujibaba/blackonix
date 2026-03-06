package agent

import (
	"blackonix/internal/ports"
	"fmt"
	"sync"
)

// ToolRegistry gerencia o registro dinâmico de AgentTools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]AgentTool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]AgentTool),
	}
}

func (r *ToolRegistry) Register(tool AgentTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (AgentTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

func (r *ToolRegistry) List() []AgentTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]AgentTool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// ToLLMTools converte todas as tools registradas para o formato da LLM.
func (r *ToolRegistry) ToLLMTools() []ports.ToolDefinition {
	tools := r.List()
	defs := make([]ports.ToolDefinition, len(tools))
	for i, t := range tools {
		defs[i] = ports.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.ParametersSchema(),
		}
	}
	return defs
}
