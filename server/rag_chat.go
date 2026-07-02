package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RAGFragment is an article fragment returned from Python.
type RAGFragment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// pythonRAGContextResponse is the body of POST /rag/context.
type pythonRAGContextResponse struct {
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Context   string        `json:"context,omitempty"`
	FewShot   string        `json:"few_shot,omitempty"`
	Category  string        `json:"category,omitempty"`
	Fragments []RAGFragment `json:"fragments,omitempty"`
}

// fetchRAGContext POSTs to Python /rag/context for article fragments and few-shot examples.
func fetchRAGContext(question, cropID string) (*pythonRAGContextResponse, error) {
	body := map[string]string{"question": question, "crop_id": cropID}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal RAG request: %w", err)
	}
	req, err := http.NewRequest("POST", config.PythonRAGURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create RAG request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := pythonHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RAG request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read RAG response: %w", err)
	}
	var out pythonRAGContextResponse
	if err := json.Unmarshal(responseBody, &out); err != nil {
		return nil, fmt.Errorf("parse RAG response: %w: %s", err, string(responseBody))
	}
	// HTTP 422 from Python = expected empty context (success:false), not a transport error.
	if resp.StatusCode == http.StatusUnprocessableEntity && !out.Success {
		return &out, nil
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != "" {
			return &out, fmt.Errorf("%s", out.Error)
		}
		return &out, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseBody))
	}
	return &out, nil
}

const ragUserPromptTpl = `<system>%s</system>
<context>%s</context>
<examples>%s</examples>
<task>Answer the user's question clearly and concisely.</task>
<constraints>
- DO NOT INVENT. If the answer is not in the context, say: "The reference materials do not contain information on your question."
- Respond only in English, clearly and without errors.
- DO NOT use "probably", "likely", "possibly", "perhaps".
- DO NOT use filler phrases like "I think", "let me answer", "let's look", or typos such as "worker" for "answer".
- If the context has specific numbers, dosages, or table data — include them in the answer.
- The answer should be thorough and include relevant details from the context.
- Finish the answer completely; do not cut off mid-sentence.
- DO NOT cite article titles, journals, authors, or publication links.
</constraints>
<output_format>
Start directly with facts, without unnecessary introductions. Be detailed and clear.
</output_format>
Question: %s
`

// buildRAGUserPrompt assembles the LLM user prompt from RAG context and few-shot examples.
func buildRAGUserPrompt(question, context, fewShot, taskIntro string) string {
	return fmt.Sprintf(ragUserPromptTpl, taskIntro, context, fewShot, question)
}

// ChatRequest is the body of deprecated POST /chat (use POST /message).
type ChatRequest struct {
	Question string `json:"question"`
	CropID   string `json:"crop_id"`
}

// ragLLMInput holds prepared RAG context and LLM messages.
type ragLLMInput struct {
	CropID      string
	Question    string
	Messages    []Message
	RAGOut      *pythonRAGContextResponse
	RetrievalMs int64
	Category    string
}

// buildRAGLLMMessages runs retrieval and builds the LLM prompt (without calling the LLM).
func buildRAGLLMMessages(q, cropID string, history []Message, sessionID string) (*ragLLMInput, string, bool, error) {
	q = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(q, "\r", " "), "\n", " "))
	if q == "" {
		return nil, "Empty question", false, nil
	}

	cropID, err := normalizeCropID(cropID)
	if err != nil {
		return nil, publicAPIError(err), false, nil
	}
	if err := requireRAGEnabled(cropID); err != nil {
		return nil, publicAPIError(err), false, nil
	}

	retrievalStart := time.Now()
	ragOut, err := fetchRAGContext(q, cropID)
	retrievalMs := time.Since(retrievalStart).Milliseconds()
	if err != nil {
		log.Printf("RAG fetch error: %v", err)
		msg := publicAPIError(err)
		if ragOut != nil && ragOut.Error != "" {
			msg = ragOut.Error
		}
		return nil, msg, false, err
	}
	if !ragOut.Success {
		logRAGTrace(RAGTrace{
			CropID:        cropID,
			SessionID:     sessionID,
			Question:      q,
			Category:      ragOut.Category,
			FragmentCount: len(ragOut.Fragments),
			VerifyPass:    false,
			VerifyReason:  ragOut.Error,
			SoftFail:      true,
			RetrievalMs:   retrievalMs,
			TotalMs:       retrievalMs,
		})
		return nil, ragOut.Error, true, nil
	}
	if config.LLMAPIKey == "" {
		return nil, "Set LLM_API_KEY for text chat (OpenRouter / OpenAI-compatible API).", false, nil
	}

	prompts := promptsForCrop(cropID)
	userPrompt := buildRAGUserPrompt(q, ragOut.Context, ragOut.FewShot, prompts.RAGTaskIntro)
	var msgs []Message
	msgs = append(msgs, Message{
		Role:    "system",
		Content: prompts.RAGSystem,
	})
	msgs = append(msgs, history...)
	msgs = append(msgs, Message{Role: "user", Content: userPrompt})

	return &ragLLMInput{
		CropID:      cropID,
		Question:    q,
		Messages:    msgs,
		RAGOut:      ragOut,
		RetrievalMs: retrievalMs,
		Category:    ragOut.Category,
	}, "", false, nil
}

// finalizeRAGAnswer cleans the LLM reply, appends disclaimer, and verifies numbers against fragments.
func finalizeRAGAnswer(raw string, input *ragLLMInput, sessionID string) (answer string, ok bool, verifyPass bool, verifyReason string) {
	answer = cleanRAGAnswer(raw)
	answer = appendRAGDisclaimer(answer)
	verifyPass, verifyReason = verifyRAGAnswer(answer, input.RAGOut.Fragments)
	if !verifyPass {
		return fmt.Sprintf("⚠️ The system could not verify this answer against sources. Admin note: %s\n\nWe recommend consulting an agronomist.", verifyReason), true, false, verifyReason
	}
	return answer, true, true, verifyReason
}

// answerWithRAG runs RAG + LLM with optional dialog history (user/assistant roles).
// sessionID is used for RAG logs (may be empty for POST /chat).
func answerWithRAG(q, cropID string, history []Message, sessionID string) (answer string, success bool, errMsg string, ragSoftFail bool, trace RAGTrace) {
	input, errMsg, ragSoft, err := buildRAGLLMMessages(q, cropID, history, sessionID)
	if err != nil {
		return "", false, errMsg, false, trace
	}
	if ragSoft {
		return "", false, errMsg, true, trace
	}
	if errMsg != "" {
		return "", false, errMsg, false, trace
	}

	llmStart := time.Now()
	raw, err := callLLMCompletion(input.Messages)
	llmMs := time.Since(llmStart).Milliseconds()
	if err != nil {
		recordLLMError()
		log.Printf("LLM chat error: %v", err)
		return "", false, publicAPIError(err), false, trace
	}
	answer, ok, verifyPass, verifyReason := finalizeRAGAnswer(raw, input, sessionID)
	trace = RAGTrace{
		CropID:        input.CropID,
		SessionID:     sessionID,
		Question:      input.Question,
		Category:      input.Category,
		FragmentCount: len(input.RAGOut.Fragments),
		RetrievalMs:   input.RetrievalMs,
		LLMMs:         llmMs,
		VerifyPass:    verifyPass,
		VerifyReason:  verifyReason,
		SoftFail:      !verifyPass,
	}
	if !ok {
		return "", false, "Could not form an answer", false, trace
	}
	return answer, true, "", false, trace
}

// handleChat is deprecated POST /chat (RAG + LLM without session). Web App uses POST /message.
func handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid JSON (field question required)",
		})
		return
	}
	q := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(req.Question, "\r", " "), "\n", " "))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Empty question"})
		return
	}

	answer, ok, errMsg, ragSoft, trace := answerWithRAG(q, req.CropID, nil, "")
	if ragSoft {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errMsg})
		return
	}
	if errMsg != "" && !ok {
		if strings.Contains(errMsg, "LLM_API_KEY") {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": errMsg})
		return
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": errMsg})
		return
	}
	trace.TotalMs = trace.RetrievalMs + trace.LLMMs
	logRAGTrace(trace)
	c.JSON(http.StatusOK, gin.H{"success": true, "answer": answer})
}
