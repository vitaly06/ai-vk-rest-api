package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vitaly06/ai-vk-bot/internal/config"
	"github.com/vitaly06/ai-vk-bot/internal/models"
)

// Service — клиент к AI провайдеру (OpenAI-совместимый API)
type Service struct {
	cfg    config.AIConfig
	client *http.Client
}

func New(cfg config.AIConfig) *Service {
	return &Service{
		cfg:    cfg,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model     string             `json:"model"`
	Messages  []models.AIMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete отправляет историю диалога в AI и возвращает ответ
func (s *Service) Complete(ctx context.Context, history []models.AIMessage) (string, error) {
	messages := make([]models.AIMessage, 0, len(history)+1)
	// Системный промпт всегда первым
	if s.cfg.SystemPrompt != "" {
		messages = append(messages, models.AIMessage{
			Role:    "system",
			Content: s.cfg.SystemPrompt,
		})
	}
	messages = append(messages, history...)

	model := s.cfg.Model
	if strings.EqualFold(s.cfg.Provider, "yandex") {
		model = resolveYandexModel(model, s.cfg.ProjectID)
	}

	body, err := json.Marshal(chatRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: s.cfg.MaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	if strings.EqualFold(s.cfg.Provider, "yandex") && s.cfg.ProjectID != "" {
		req.Header.Set("OpenAI-Project", s.cfg.ProjectID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: read response: %w", err)
	}

	// Some providers (e.g. Gemini OpenAI-compat layer) wrap error responses in
	// a JSON array [{"error":{...}}] instead of a plain object.
	if len(raw) > 0 && raw[0] == '[' {
		var arr []chatResponse
		if jsonErr := json.Unmarshal(raw, &arr); jsonErr == nil && len(arr) > 0 {
			if arr[0].Error != nil {
				return "", fmt.Errorf("ai: provider error: %s", arr[0].Error.Message)
			}
			if len(arr[0].Choices) > 0 {
				return arr[0].Choices[0].Message.Content, nil
			}
		}
		return "", fmt.Errorf("ai: unexpected array response (status %d): %.200s", resp.StatusCode, raw)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("ai: decode response (status %d): %.200s", resp.StatusCode, raw)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("ai: provider error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("ai: empty response")
	}
	return cr.Choices[0].Message.Content, nil
}

func resolveYandexModel(model, folderID string) string {
	if strings.HasPrefix(model, "gpt://") {
		return model
	}
	if folderID == "" {
		return model
	}
	return "gpt://" + folderID + "/" + strings.TrimPrefix(model, "/")
}
