package agent

import (
	"blackonix/internal/domain"
	"blackonix/internal/ports"
	"blackonix/internal/repository"
	"context"
	"fmt"
	"log"
)

const maxContextMessages = 30

// Orchestrator é o "cérebro" do sistema. Recebe uma mensagem, decide o fluxo
// (BOT vs HUMAN) e orquestra LLM + Tool Calls.
type Orchestrator struct {
	registry    *ToolRegistry
	llm         ports.LLMClient
	sessionRepo repository.SessionRepository
	systemPrompt string
}

func NewOrchestrator(
	registry *ToolRegistry,
	llm ports.LLMClient,
	sessionRepo repository.SessionRepository,
	systemPrompt string,
) *Orchestrator {
	return &Orchestrator{
		registry:     registry,
		llm:          llm,
		sessionRepo:  sessionRepo,
		systemPrompt: systemPrompt,
	}
}

// ProcessMessage recebe a mensagem do usuário, envia para a LLM com tools,
// executa tool calls se necessário e retorna a resposta final.
func (o *Orchestrator) ProcessMessage(ctx context.Context, session *domain.Session, userMessage string) (string, error) {
	// Adiciona a mensagem do usuário ao contexto
	session.ContextMemory = append(session.ContextMemory, domain.ContextMessage{
		Role:    "user",
		Content: userMessage,
	})

	// Trunca histórico para evitar crescimento ilimitado
	if len(session.ContextMemory) > maxContextMessages {
		session.ContextMemory = session.ContextMemory[len(session.ContextMemory)-maxContextMessages:]
	}

	// Monta as mensagens para a LLM
	messages := o.buildMessages(session)
	tools := o.registry.ToLLMTools()

	// Loop de tool calling (máximo 5 iterações para evitar loops infinitos)
	for i := 0; i < 5; i++ {
		resp, err := o.llm.ChatCompletion(ctx, messages, tools)
		if err != nil {
			return "", fmt.Errorf("llm completion: %w", err)
		}

		// Se não há tool calls, retorna a resposta de texto
		if len(resp.ToolCalls) == 0 {
			session.ContextMemory = append(session.ContextMemory, domain.ContextMessage{
				Role:    "assistant",
				Content: resp.Content,
			})
			if err := o.sessionRepo.Update(ctx, session); err != nil {
				log.Printf("failed to update session context: %v", err)
			}
			return resp.Content, nil
		}

		// Adiciona a resposta do assistant (com tool calls) ao histórico
		messages = append(messages, ports.LLMMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Executa cada tool call
		for _, tc := range resp.ToolCalls {
			result, err := o.executeTool(ctx, tc)
			if err != nil {
				result = fmt.Sprintf("Error executing tool %s: %v", tc.Name, err)
			}

			messages = append(messages, ports.LLMMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return "", fmt.Errorf("max tool call iterations exceeded")
}

func (o *Orchestrator) buildMessages(session *domain.Session) []ports.LLMMessage {
	messages := []ports.LLMMessage{
		{Role: "system", Content: o.systemPrompt},
	}

	for _, m := range session.ContextMemory {
		messages = append(messages, ports.LLMMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return messages
}

func (o *Orchestrator) executeTool(ctx context.Context, tc ports.ToolCall) (string, error) {
	tool, err := o.registry.Get(tc.Name)
	if err != nil {
		return "", err
	}
	return tool.Execute(ctx, tc.Parameters)
}
