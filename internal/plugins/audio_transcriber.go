package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"blackonix/internal/ports"
)

const whisperURL = "https://api.openai.com/v1/audio/transcriptions"

// AudioTranscriberTool transcreve áudios do WhatsApp usando a API Whisper da OpenAI.
type AudioTranscriberTool struct {
	apiKey     string
	metaAPI    ports.MetaAPI
	httpClient *http.Client
}

func NewAudioTranscriberTool(apiKey string, metaAPI ports.MetaAPI) *AudioTranscriberTool {
	return &AudioTranscriberTool{
		apiKey:     apiKey,
		metaAPI:    metaAPI,
		httpClient: &http.Client{},
	}
}

func (t *AudioTranscriberTool) Name() string {
	return "transcribe_audio"
}

func (t *AudioTranscriberTool) Description() string {
	return "Transcreve uma mensagem de áudio recebida do WhatsApp para texto usando IA (Whisper). Use quando o cliente enviar um áudio."
}

func (t *AudioTranscriberTool) ParametersSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"media_id": map[string]interface{}{
				"type":        "string",
				"description": "ID da mídia do WhatsApp (fornecido automaticamente pelo sistema)",
			},
			"meta_token": map[string]interface{}{
				"type":        "string",
				"description": "Token de acesso da Meta (fornecido automaticamente pelo sistema)",
			},
		},
		"required": []string{"media_id", "meta_token"},
	}
}

func (t *AudioTranscriberTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	mediaID, _ := params["media_id"].(string)
	metaToken, _ := params["meta_token"].(string)

	if mediaID == "" {
		return "", fmt.Errorf("media_id is required")
	}
	if metaToken == "" {
		return "", fmt.Errorf("meta_token is required")
	}

	// 1. Baixar o áudio da Meta
	audioData, err := t.metaAPI.DownloadMedia(ctx, metaToken, mediaID)
	if err != nil {
		return "", fmt.Errorf("download audio: %w", err)
	}

	// 2. Enviar para o Whisper
	transcript, err := t.transcribe(ctx, audioData)
	if err != nil {
		return "", fmt.Errorf("transcribe audio: %w", err)
	}

	return transcript, nil
}

func (t *AudioTranscriberTool) transcribe(ctx context.Context, audioData []byte) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Campo "file" (o Whisper aceita ogg/opus que é o formato do WhatsApp)
	part, err := writer.CreateFormFile("file", "audio.ogg")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(audioData)); err != nil {
		return "", fmt.Errorf("write audio data: %w", err)
	}

	// Campo "model"
	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}

	// Campo "language" (português)
	if err := writer.WriteField("language", "pt"); err != nil {
		return "", fmt.Errorf("write language field: %w", err)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, whisperURL, &buf)
	if err != nil {
		return "", fmt.Errorf("create whisper request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode whisper response: %w", err)
	}

	return result.Text, nil
}
