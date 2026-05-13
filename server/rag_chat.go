package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RAGFragment — фрагмент статьи из Python.
type RAGFragment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// pythonRAGContextResponse — ответ POST /rag/context.
type pythonRAGContextResponse struct {
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Context   string        `json:"context,omitempty"`
	FewShot   string        `json:"few_shot,omitempty"`
	Category  string        `json:"category,omitempty"`
	Fragments []RAGFragment `json:"fragments,omitempty"`
}

func fetchRAGContext(question string) (*pythonRAGContextResponse, error) {
	body := map[string]string{"question": question}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal RAG request: %w", err)
	}
	req, err := http.NewRequest("POST", config.PythonRAGURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create RAG request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
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
	if resp.StatusCode != http.StatusOK {
		if out.Error != "" {
			return &out, fmt.Errorf("%s", out.Error)
		}
		return &out, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseBody))
	}
	return &out, nil
}

const ragUserPromptTpl = `<system>Ты — грамотный агроном по яблоням. Отвечай строго на основе контекста. Используй правильный русский язык, избегай грамматических ошибок, согласуй слова.</system>
<context>%s</context>
<examples>%s</examples>
<task>Ответь на вопрос пользователя чётко, по делу, грамотно.</task>
<constraints>
- НЕ ВЫДУМЫВАЙ. Если ответа нет в контексте — скажи: "В предоставленных статьях нет информации по вашему вопросу."
- Отвечай только на русском языке, литературно, без ошибок.
- НЕ используй слова "скорее всего", "вероятно", "возможно", "наверное".
- НЕ используй слова "аботчик", "Я думаю", "мне нужно ответить", "давайте посмотрим".
- Если в контексте есть конкретные цифры, дозировки, табличные данные — обязательно включи их в ответ.
- Ответ должен быть развёрнутым, полезным, содержать все детали из источника.
- Завершай ответ полностью, не обрывай на полуслове.
- В конце ответа укажи источник: Источник: "Название статьи".
</constraints>
<output_format>
Ответ должен начинаться сразу с факта, без лишних вступлений. Будь подробным и грамотным.
</output_format>
Вопрос: %s
`

func buildRAGUserPrompt(question, context, fewShot string) string {
	return fmt.Sprintf(ragUserPromptTpl, context, fewShot, question)
}

var (
	reNumberWord = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	reMultiSpace = regexp.MustCompile(`\s+`)
	reThink      = regexp.MustCompile(`(?i)</?think>`)
	reAnswerTag  = regexp.MustCompile(`(?i)</?answer>`)
	reSystemTag  = regexp.MustCompile(`(?i)</?system>`)
	reAbot       = regexp.MustCompile(`(?i)\bаботчик\b`)
	reIntro      = regexp.MustCompile(`(?i)^(Хорошо|Давайте посмотрим|Итак|Я думаю|мне нужно ответить|Из контекста видно|Теперь я понимаю|Из таблицы видно)[,:.]?\s*`)
)

func extractNumbersFromText(s string) []float64 {
	s = strings.ReplaceAll(s, ",", ".")
	var out []float64
	for _, m := range reNumberWord.FindAllString(s, -1) {
		v, err := strconv.ParseFloat(m, 64)
		if err == nil {
			out = append(out, v)
		}
	}
	return out
}

func cleanRAGAnswer(text string) string {
	if text == "" {
		return "Ответ не сформирован корректно."
	}
	text = reThink.ReplaceAllString(text, "")
	text = reAnswerTag.ReplaceAllString(text, "")
	text = reSystemTag.ReplaceAllString(text, "")
	text = reAbot.ReplaceAllString(text, "")
	text = reIntro.ReplaceAllString(text, "")
	text = strings.TrimSpace(reMultiSpace.ReplaceAllString(text, " "))
	if text == "" {
		return "Ответ не сформирован корректно."
	}
	return text
}

func enforceRAGSource(answer string, fragments []RAGFragment) string {
	if strings.Contains(answer, "Источник:") {
		return answer
	}
	if len(fragments) > 0 {
		src := fragments[0].Filename
		return fmt.Sprintf("%s\n\nИсточник: \"%s\"", strings.TrimSpace(answer), src)
	}
	return answer
}

func verifyRAGAnswer(answer string, fragments []RAGFragment) (bool, string) {
	if answer == "" {
		return false, "Ответ отсутствует"
	}
	if !strings.Contains(answer, "Источник:") {
		return false, "В ответе отсутствует ссылка на источник (формат 'Источник: \"Название статьи\"')."
	}
	var ctx strings.Builder
	for _, f := range fragments {
		ctx.WriteString(f.Content)
		ctx.WriteByte('\n')
	}
	numsAns := extractNumbersFromText(answer)
	if len(numsAns) == 0 {
		return true, "Верификация пройдена"
	}
	numsCtx := extractNumbersFromText(ctx.String())
	var missing []float64
	for _, n := range numsAns {
		found := false
		for _, c := range numsCtx {
			if math.Abs(n-c) < 0.01 {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return false, fmt.Sprintf("Число(а) %v не найдены в источниках.", missing)
	}
	return true, "Верификация пройдена"
}

// ChatRequest — тело POST /chat от Web App.
type ChatRequest struct {
	Question string `json:"question"`
}

// answerWithRAG выполняет RAG + LLM с опциональной историей диалога (роли user/assistant).
// ragSoftFail: контентная ошибка RAG (HTTP 200 от Python, нет фрагментов) — отдавать клиенту 200 success:false.
func answerWithRAG(q string, history []Message) (answer string, success bool, errMsg string, ragSoftFail bool) {
	q = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(q, "\r", " "), "\n", " "))
	if q == "" {
		return "", false, "Пустой вопрос", false
	}

	ragOut, err := fetchRAGContext(q)
	if err != nil {
		log.Printf("RAG fetch error: %v", err)
		msg := err.Error()
		if ragOut != nil && ragOut.Error != "" {
			msg = ragOut.Error
		}
		return "", false, msg, false
	}
	if !ragOut.Success {
		return "", false, ragOut.Error, true
	}
	if config.LLMAPIKey == "" {
		return "", false, "Для текстового чата задайте LLM_API_KEY (OpenRouter / OpenAI-совместимый API).", false
	}

	userPrompt := buildRAGUserPrompt(q, ragOut.Context, ragOut.FewShot)
	var msgs []Message
	msgs = append(msgs, Message{
		Role:    "system",
		Content: "Ты — грамотный агроном по яблоням. Следуй инструкциям и ограничениям в последнем сообщении пользователя (блок с контекстом и вопросом). Отвечай на русском.",
	})
	msgs = append(msgs, history...)
	msgs = append(msgs, Message{Role: "user", Content: userPrompt})

	raw, err := callLLMCompletion(msgs)
	if err != nil {
		log.Printf("LLM chat error: %v", err)
		return "", false, err.Error(), false
	}
	answer = cleanRAGAnswer(raw)
	answer = enforceRAGSource(answer, ragOut.Fragments)
	passed, reason := verifyRAGAnswer(answer, ragOut.Fragments)
	if !passed {
		return fmt.Sprintf("⚠️ Система не смогла подтвердить ответ источниками. Сообщение администратору: %s\n\nРекомендуем обратиться к агроному.", reason), true, "", false
	}
	return answer, true, "", false
}

// handleChat: Python отдаёт контекст RAG, Go собирает промпт и вызывает LLM (без истории).
func handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Некорректный JSON (нужно поле question)",
		})
		return
	}
	q := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(req.Question, "\r", " "), "\n", " "))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Пустой вопрос"})
		return
	}

	answer, ok, errMsg, ragSoft := answerWithRAG(q, nil)
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
	c.JSON(http.StatusOK, gin.H{"success": true, "answer": answer})
}
