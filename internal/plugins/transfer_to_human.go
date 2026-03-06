package plugins

import (
	"blackonix/internal/core/state"
	"blackonix/internal/domain"
	"blackonix/internal/ports"
	"context"
	"fmt"
)

// TransferToHumanTool altera o State da Session para HUMAN e notifica o Rocket.Chat.
type TransferToHumanTool struct {
	stateMachine *state.Machine
	rocketChat   ports.RocketChatAPI
	session      *domain.Session
	tenant       *domain.Tenant
	contact      *domain.Contact
}

func NewTransferToHumanTool(
	sm *state.Machine,
	rc ports.RocketChatAPI,
	session *domain.Session,
	tenant *domain.Tenant,
	contact *domain.Contact,
) *TransferToHumanTool {
	return &TransferToHumanTool{
		stateMachine: sm,
		rocketChat:   rc,
		session:      session,
		tenant:       tenant,
		contact:      contact,
	}
}

func (t *TransferToHumanTool) Name() string {
	return "transfer_to_human"
}

func (t *TransferToHumanTool) Description() string {
	return "Transfere a conversa para um atendente humano no Rocket.Chat. Use quando o cliente solicitar falar com um humano ou quando o bot não conseguir resolver o problema."
}

func (t *TransferToHumanTool) ParametersSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"department": map[string]interface{}{
				"type":        "string",
				"description": "Departamento para transferência (ex: vendas, suporte)",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Motivo da transferência para o atendente",
			},
		},
		"required": []string{"department"},
	}
}

func (t *TransferToHumanTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	department, _ := params["department"].(string)
	if department == "" {
		department = "general"
	}

	reason, _ := params["reason"].(string)

	// Transição de estado: BOT -> HUMAN
	if err := t.stateMachine.TransitionTo(ctx, t.session, domain.SessionStateHuman); err != nil {
		return "", fmt.Errorf("state transition failed: %w", err)
	}

	t.session.ActiveDepartment = department

	// Notifica o Rocket.Chat
	msg := fmt.Sprintf("[Transferido pelo bot] Cliente: %s | Motivo: %s", t.contact.Name, reason)
	if err := t.rocketChat.SendMessage(
		ctx,
		t.tenant.RocketChatURL,
		t.tenant.RocketChatToken,
		department,
		t.contact.Name,
		t.contact.PhoneNumber,
		msg,
	); err != nil {
		return "", fmt.Errorf("rocketchat notification failed: %w", err)
	}

	return fmt.Sprintf("Conversa transferida para o departamento '%s'. Um atendente humano irá assumir em breve.", department), nil
}
