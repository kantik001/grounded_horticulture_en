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

// POST в Python /rag/context: фрагменты статей и few-shot для промпта.
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

const ragUserPromptTpl = `<system>%s</system>
<context>%s</context>
<examples>%s</examples>
<task>Ответь на вопрос пользователя чётко, по делу, грамотно.</task>
<constraints>
- НЕ ВЫДУМЫВАЙ. Если ответа нет в контексте — скажи: "В справочных материалах нет информации по вашему вопросу."
- Отвечай только на русском языке, литературно, без ошибок.
- НЕ используй слова "скорее всего", "вероятно", "возможно", "наверное".
- НЕ используй слова "аботчик", "Я думаю", "мне нужно ответить", "давайте посмотрим".
- Если в контексте есть конкретные цифры, дозировки, табличные данные — обязательно включи их в ответ.
- Ответ должен быть развёрнутым, полезным, содержать все детали из контекста.
- Завершай ответ полностью, не обрывай на полуслове.
- НЕ указывай названия статей, журналов, авторов и ссылки на публикации.
</constraints>
<output_format>
Ответ должен начинаться сразу с факта, без лишних вступлений. Будь подробным и грамотным.
</output_format>
Вопрос: %s
`

// Собирает user-промпт для LLM из контекста RAG и few-shot примеров.
func buildRAGUserPrompt(question, context, fewShot, taskIntro string) string {
	return fmt.Sprintf(ragUserPromptTpl, taskIntro, context, fewShot, question)
}

// ragAnswerDisclaimer — общий дисклеймер в конце ответа (без названий статей).
const ragAnswerDisclaimer = "Справочная информация из базы знаний. Не заменяет очный осмотр агронома; решения по препаратам — с учётом инструкций и законодательства."

var (
	reNumberWord = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	reMultiSpace = regexp.MustCompile(`\s+`)
	reThink      = regexp.MustCompile(`(?i)</?think>`)
	reAnswerTag  = regexp.MustCompile(`(?i)</?answer>`)
	reSystemTag  = regexp.MustCompile(`(?i)</?system>`)
	reAbot       = regexp.MustCompile(`(?i)\bаботчик\b`)
	reIntro      = regexp.MustCompile(`(?i)^(Хорошо|Давайте посмотрим|Итак|Я думаю|мне нужно ответить|Из контекста видно|Теперь я понимаю|Из таблицы видно)[,:.]?\s*`)
	reSourceLine = regexp.MustCompile(`(?im)^\s*Источник:.*\n?`)
)

// Извлекает числа из текста для верификации ответа RAG.
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

// Убирает служебные теги и вводные фразы из ответа LLM.
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

// Удаляет строки «Источник:» из ответа перед показом пользователю.
func stripSourceAttribution(answer string) string {
	s := reSourceLine.ReplaceAllString(answer, "")
	return strings.TrimSpace(reMultiSpace.ReplaceAllString(s, " "))
}

// Добавляет дисклеймер в конец ответа RAG.
func appendRAGDisclaimer(answer string) string {
	body := stripSourceAttribution(answer)
	if body == "" {
		return ragAnswerDisclaimer
	}
	if strings.Contains(body, "Не заменяет очный осмотр агронома") {
		return body
	}
	return body + "\n\n" + ragAnswerDisclaimer
}

// Текст ответа без дисклеймера и источников — для проверки чисел.
func answerBodyForVerification(answer string) string {
	s := stripSourceAttribution(answer)
	s = strings.ReplaceAll(s, ragAnswerDisclaimer, "")
	return strings.TrimSpace(s)
}

// Проверяет, что все числа в ответе есть во фрагментах статей.
func verifyRAGAnswer(answer string, fragments []RAGFragment) (bool, string) {
	if answer == "" {
		return false, "Ответ отсутствует"
	}
	var ctx strings.Builder
	for _, f := range fragments {
		ctx.WriteString(f.Content)
		ctx.WriteByte('\n')
	}
	numsAns := extractNumbersFromText(answerBodyForVerification(answer))
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
	CropID   string `json:"crop_id"`
}

// answerWithRAG выполняет RAG + LLM с опциональной историей диалога (роли user/assistant).
func answerWithRAG(q, cropID string, history []Message) (answer string, success bool, errMsg string, ragSoftFail bool) {
	q = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(q, "\r", " "), "\n", " "))
	if q == "" {
		return "", false, "Пустой вопрос", false
	}

	cropID, err := normalizeCropID(cropID)
	if err != nil {
		return "", false, err.Error(), false
	}

	ragOut, err := fetchRAGContext(q, cropID)
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

	prompts := promptsForCrop(cropID)
	userPrompt := buildRAGUserPrompt(q, ragOut.Context, ragOut.FewShot, prompts.RAGTaskIntro)
	var msgs []Message
	msgs = append(msgs, Message{
		Role:    "system",
		Content: prompts.RAGSystem,
	})
	msgs = append(msgs, history...)
	msgs = append(msgs, Message{Role: "user", Content: userPrompt})

	raw, err := callLLMCompletion(msgs)
	if err != nil {
		log.Printf("LLM chat error: %v", err)
		return "", false, err.Error(), false
	}
	answer = cleanRAGAnswer(raw)
	answer = appendRAGDisclaimer(answer)
	passed, reason := verifyRAGAnswer(answer, ragOut.Fragments)
	if !passed {
		return fmt.Sprintf("⚠️ Система не смогла подтвердить ответ источниками. Сообщение администратору: %s\n\nРекомендуем обратиться к агроному.", reason), true, "", false
	}
	return answer, true, "", false
}

// POST /chat: RAG + LLM без сессии (одиночный вопрос из Web App).
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

	answer, ok, errMsg, ragSoft := answerWithRAG(q, req.CropID, nil)
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
