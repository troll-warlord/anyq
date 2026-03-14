package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type anthropicProvider struct {
	apiKey string
	model  string
}

func (p *anthropicProvider) Translate(schema, request string) (string, error) {
	userPrompt, err := BuildUserPrompt(schema, request)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]interface{}{
		"model":      p.model,
		"max_tokens": 256,
		"system":     SystemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic error %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("anthropic: parse response: %w", err)
	}
	if len(result.Content) == 0 {
		return ".", nil
	}
	return CleanExpr(result.Content[0].Text), nil
}
