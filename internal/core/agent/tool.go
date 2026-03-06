package agent

import "context"

// AgentTool é a interface central do sistema de plugins.
// Qualquer funcionalidade expansível (PIX, Whisper, Copiloto) implementa esta interface.
type AgentTool interface {
	Name() string
	Description() string
	ParametersSchema() interface{}
	Execute(ctx context.Context, params map[string]interface{}) (string, error)
}
