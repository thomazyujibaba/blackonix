package ports

import "context"

// RocketChatAPI define a interface para comunicação com o Rocket.Chat Omnichannel.
type RocketChatAPI interface {
	SendMessage(ctx context.Context, baseURL, token, department, visitorName, visitorPhone, text string) error
	TransferToAgent(ctx context.Context, baseURL, token, department, visitorPhone string) error
}
