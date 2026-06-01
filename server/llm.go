package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMRequest represents the request to LLM API.
type LLMRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message represents a chat message for LLM.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse represents the response from LLM API.
type LLMResponse struct {
	Choices []Choice `json:"choices"`
}

// Choice represents a choice in LLM response.
type Choice struct {
	Message Message `json:"message"`
}

// callLLMCompletion отправляет запрос в LLM API (OpenAI-совместимый).
func callLLMCompletion(messages []Message) (string, error) {
	if config.LLMAPIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}
	llmReq := &LLMRequest{
		Model:    config.LLMModel,
		Messages: messages,
	}
	requestBody, err := json.Marshal(llmReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %v", err)
	}
	req, err := http.NewRequest("POST", config.LLMBaseURL+"/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.LLMAPIKey))
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send LLM request: %v", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read LLM response: %v", err)
	}
	var llmResp LLMResponse
	if err := json.Unmarshal(responseBody, &llmResp); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %v", err)
	}
	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}
	return llmResp.Choices[0].Message.Content, nil
}
