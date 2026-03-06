package state

import (
	"blackonix/internal/domain"
	"blackonix/internal/repository"
	"context"
	"fmt"
)

// Machine gerencia as transições de estado da sessão (BOT <-> HUMAN).
type Machine struct {
	sessionRepo repository.SessionRepository
}

func NewMachine(sessionRepo repository.SessionRepository) *Machine {
	return &Machine{sessionRepo: sessionRepo}
}

// TransitionTo altera o estado da sessão e persiste no banco.
func (m *Machine) TransitionTo(ctx context.Context, session *domain.Session, newState domain.SessionState) error {
	oldState := session.State

	if oldState == newState {
		return nil
	}

	if !isValidTransition(oldState, newState) {
		return fmt.Errorf("invalid state transition: %s -> %s", oldState, newState)
	}

	session.State = newState
	return m.sessionRepo.Update(ctx, session)
}

// IsBot retorna true se a sessão está no modo BOT.
func (m *Machine) IsBot(session *domain.Session) bool {
	return session.State == domain.SessionStateBot
}

// IsHuman retorna true se a sessão está no modo HUMAN.
func (m *Machine) IsHuman(session *domain.Session) bool {
	return session.State == domain.SessionStateHuman
}

func isValidTransition(from, to domain.SessionState) bool {
	transitions := map[domain.SessionState][]domain.SessionState{
		domain.SessionStateBot:   {domain.SessionStateHuman},
		domain.SessionStateHuman: {domain.SessionStateBot},
	}

	for _, valid := range transitions[from] {
		if valid == to {
			return true
		}
	}
	return false
}
