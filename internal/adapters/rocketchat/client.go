package rocketchat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"blackonix/internal/ports"
)

type client struct {
	httpClient *http.Client
}

func NewRocketChatClient() ports.RocketChatAPI {
	return &client{
		httpClient: &http.Client{},
	}
}

func (c *client) SendMessage(ctx context.Context, baseURL, token, department, visitorName, visitorPhone, text string) error {
	url := baseURL + "/api/v1/livechat/message"

	payload := map[string]interface{}{
		"token":      visitorPhone,
		"rid":        department,
		"msg":        text,
		"visitor": map[string]string{
			"name":  visitorName,
			"phone": visitorPhone,
			"token": visitorPhone,
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal rocketchat payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send rocketchat message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("rocketchat API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *client) TransferToAgent(ctx context.Context, baseURL, token, department, visitorPhone string) error {
	url := baseURL + "/api/v1/livechat/room.transfer"

	payload := map[string]interface{}{
		"token":      visitorPhone,
		"department": department,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal transfer payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("transfer to agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("rocketchat transfer API returned status %d", resp.StatusCode)
	}

	return nil
}
