package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"blackonix/internal/ports"
)

const graphAPIURL = "https://graph.facebook.com/v21.0"

type client struct {
	httpClient *http.Client
}

func NewMetaClient() ports.MetaAPI {
	return &client{
		httpClient: &http.Client{},
	}
}

func (c *client) SendTextMessage(ctx context.Context, token, phoneNumberID, to, body string) error {
	url := fmt.Sprintf("%s/%s/messages", graphAPIURL, phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"body": body,
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("meta API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *client) VerifyWebhook(mode, token, challenge, verifyToken string) (string, error) {
	if mode == "subscribe" && token == verifyToken {
		return challenge, nil
	}
	return "", fmt.Errorf("webhook verification failed")
}
