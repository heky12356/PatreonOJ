package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"patreonoj/internal/config"
)

var ErrAINotImplemented = errors.New("ai implementation is not ready")

// AIService provides AI integration methods.
type AIService struct {
	Config config.AIConfig
	Client *http.Client
}

func NewAIService(cfg config.AIConfig) *AIService {
	return &AIService{
		Config: cfg,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func (s *AIService) callAI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !s.Config.Enabled {
		return "", fmt.Errorf("AI service is disabled")
	}

	reqBody := ChatRequest{
		Model:       s.Config.Model,
		Temperature: s.Config.Temperature,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Config.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if s.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.Config.APIKey)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from AI")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// GenerateQuestionTags generates knowledge tags for a question payload.
// Current stage keeps this as an interface placeholder.
func (s *AIService) GenerateQuestionTags(ctx context.Context, questionPayload string) ([]string, error) {
	_ = ctx
	_ = questionPayload
	return nil, ErrAINotImplemented
}

func cleanAIResponse(content string) string {
	content = strings.TrimSpace(content)

	if idx := strings.Index(content, "</think>"); idx != -1 {
		content = content[idx+len("</think>"):]
		content = strings.TrimSpace(content)
	}

	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}
