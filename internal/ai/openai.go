package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 120 * time.Second}

type openAIProvider struct {
	apiKey string
	model  string
}

func (p *openAIProvider) Translate(schema, request string) (string, error) {
	userPrompt, err := BuildUserPrompt(schema, request)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]interface{}{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": SystemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0,
		"max_tokens":  256,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai error %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("openai: parse response: %w", err)
	}
	if len(result.Choices) == 0 {
		return ".", nil
	}
	return CleanExpr(result.Choices[0].Message.Content), nil
}
