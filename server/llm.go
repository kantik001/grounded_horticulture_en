package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMRequest is the body for an OpenAI-compatible chat/completions request.
type LLMRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

// Message is one dialog message for the LLM.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse is a chat/completions response.
type LLMResponse struct {
	Choices []Choice `json:"choices"`
}

// Choice is one model response variant.
type Choice struct {
	Message Message `json:"message"`
}

type llmStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

var llmHTTPClient = &http.Client{
	Timeout:   120 * time.Second,
	Transport: outboundTransport,
}

// callLLMCompletion sends a request to the LLM API (OpenAI-compatible).
func callLLMCompletion(ctx context.Context, messages []Message) (string, error) {
	return callLLMCompletionStream(ctx, messages, nil)
}

// callLLMCompletionStream streams LLM tokens; onDelta is called for each chunk.
func callLLMCompletionStream(ctx context.Context, messages []Message, onDelta func(string) error) (string, error) {
	if config.LLMAPIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}
	llmReq := &LLMRequest{
		Model:    config.LLMModel,
		Messages: messages,
		Stream:   onDelta != nil,
	}
	requestBody, err := json.Marshal(llmReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", config.LLMBaseURL+"/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.LLMAPIKey))
	req.Header.Set("Accept", "text/event-stream")

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send LLM request: %v", err)
	}
	defer resp.Body.Close()

	if onDelta == nil {
		return readLLMCompletionBody(resp)
	}
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", llmHTTPError(resp.StatusCode, responseBody)
	}
	return readLLMStream(resp.Body, onDelta)
}

// readLLMCompletionBody parses a non-streaming chat/completions response.
func readLLMCompletionBody(resp *http.Response) (string, error) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read LLM response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", llmHTTPError(resp.StatusCode, responseBody)
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

// llmHTTPError builds an error from a non-200 LLM API response.
func llmHTTPError(status int, body []byte) error {
	var errPayload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errPayload) == nil && errPayload.Error.Message != "" {
		return fmt.Errorf("LLM API HTTP %d: %s", status, errPayload.Error.Message)
	}
	return fmt.Errorf("LLM API HTTP %d: %s", status, string(body))
}

// readLLMStream reads SSE lines, concatenating deltas and invoking onDelta per chunk.
func readLLMStream(body io.Reader, onDelta func(string) error) (string, error) {
	var full strings.Builder
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		content, err := parseLLMStreamData(data)
		if err != nil {
			return full.String(), err
		}
		if content == "" {
			continue
		}
		full.WriteString(content)
		if onDelta != nil {
			if err := onDelta(content); err != nil {
				return full.String(), err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("failed to read LLM stream: %v", err)
	}
	if full.Len() == 0 {
		return "", fmt.Errorf("empty LLM stream response")
	}
	return full.String(), nil
}

// parseLLMStreamData extracts the delta content from one SSE data payload.
func parseLLMStreamData(data string) (string, error) {
	var chunk llmStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", fmt.Errorf("failed to parse LLM stream chunk: %v", err)
	}
	if len(chunk.Choices) == 0 {
		return "", nil
	}
	return chunk.Choices[0].Delta.Content, nil
}
