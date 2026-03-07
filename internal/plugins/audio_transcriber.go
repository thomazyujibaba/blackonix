package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"blackonix/internal/domain"
	"blackonix/internal/ports"
)

const (
	whisperURL         = "https://api.openai.com/v1/audio/transcriptions"
	whisperHTTPTimeout = 45 * time.Second
)

type AudioTranscriberTool struct {
	apiKey     string
	httpClient *http.Client
}

func NewAudioTranscriberTool(apiKey string) *AudioTranscriberTool {
	return &AudioTranscriberTool{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: whisperHTTPTimeout},
	}
}

// TranscribeFromChannel downloads audio from a channel and transcribes it.
func (t *AudioTranscriberTool) TranscribeFromChannel(ctx context.Context, ch ports.MessagingChannel, channel *domain.Channel, mediaID string) (string, error) {
	audioData, err := ch.DownloadMedia(ctx, channel, mediaID)
	if err != nil {
		return "", fmt.Errorf("download audio: %w", err)
	}

	transcript, err := t.transcribe(ctx, audioData)
	if err != nil {
		return "", fmt.Errorf("transcribe audio: %w", err)
	}

	return transcript, nil
}

func (t *AudioTranscriberTool) Name() string {
	return "transcribe_audio"
}

func (t *AudioTranscriberTool) Description() string {
	return "Transcreve uma mensagem de áudio recebida para texto usando IA (Whisper)."
}

func (t *AudioTranscriberTool) ParametersSchema() interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *AudioTranscriberTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	return "", fmt.Errorf("use TranscribeFromChannel instead")
}

func (t *AudioTranscriberTool) transcribe(ctx context.Context, audioData []byte) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.ogg")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(audioData)); err != nil {
		return "", fmt.Errorf("write audio data: %w", err)
	}

	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}

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
