package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type geminiProvider struct {
	apiKey string
	model  string
}

func (p *geminiProvider) Translate(schema, request string) (string, error) {
	userPrompt, err := BuildUserPrompt(schema, request)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]interface{}{
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]string{{"text": SystemPrompt}},
		},
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": userPrompt}},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0,
			"maxOutputTokens": 256,
		},
	})
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		p.model, p.apiKey,
	)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini error %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("gemini: parse response: %w", err)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return ".", nil
	}
	return CleanExpr(result.Candidates[0].Content.Parts[0].Text), nil
}
