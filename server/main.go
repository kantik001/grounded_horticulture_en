package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// Configuration holds application configuration
type Config struct {
	PythonServiceURL string // POST multipart → классификация изображения
	PythonRAGURL     string // POST JSON {"question"} → только retrieval (контекст) в Python
	LLMAPIKey        string
	LLMModel         string
	LLMBaseURL       string
	ServerPort       string

	TelegramBotToken       string
	TelegramAuthDisabled   bool
	TelegramInitDataMaxAge time.Duration
	CORSAllowedOrigins     []string
	RateLimitPerMinute     int
	DatabaseURL            string
	UploadDir              string
	DataDir                string
	PythonBaseURL          string
	AdminUser              string
	AdminPassword          string
	AdminSecret            string
}

// ClassificationResult represents the result from Python classifier
type ClassificationResult struct {
	Success        bool                  `json:"success"`
	Prediction     string                `json:"prediction"`
	Confidence     float64               `json:"confidence"`
	TopPredictions []PredictionCandidate `json:"top_predictions"`
	Error          string                `json:"error,omitempty"`
}

// PredictionCandidate represents a single prediction candidate
type PredictionCandidate struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

// LLMRequest represents the request to LLM API
type LLMRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message represents a chat message for LLM
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse represents the response from LLM API
type LLMResponse struct {
	Choices []Choice `json:"choices"`
}

// Choice represents a choice in LLM response
type Choice struct {
	Message Message `json:"message"`
}

// RecommendationResponse represents the final response to client
type RecommendationResponse struct {
	Success bool `json:"success"`
	ClassificationResult
	Recommendation string `json:"recommendation,omitempty"`
	Error          string `json:"error,omitempty"`
}

var config *Config

func loadConfig() *Config {
	// Try to load .env file from cwd and parent (docker / local server/)
	godotenv.Load()
	godotenv.Load("../.env")

	maxAgeSec, _ := strconv.Atoi(getEnv("TELEGRAM_INIT_DATA_MAX_AGE_SEC", "86400"))
	if maxAgeSec < 0 {
		maxAgeSec = 86400
	}
	rateLimit, _ := strconv.Atoi(getEnv("RATE_LIMIT_REQUESTS_PER_MINUTE", "30"))
	if rateLimit < 0 {
		rateLimit = 0
	}

	return &Config{
		PythonServiceURL: getEnv("CLASSIFIER_URL", "http://classifier:5000/classify"),
		PythonRAGURL:     getEnv("CLASSIFIER_RAG_URL", "http://classifier:5000/rag/context"),
		LLMAPIKey:        getEnv("LLM_API_KEY", ""),
		LLMBaseURL:       getEnv("LLM_BASE_URL", "https://openrouter.ai/api"),
		LLMModel:         getEnv("LLM_MODEL", "openrouter/free"),
		ServerPort:       getEnv("SERVER_PORT", "8080"),

		TelegramBotToken:       getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramAuthDisabled:   strings.EqualFold(getEnv("TELEGRAM_AUTH_DISABLED", "false"), "true"),
		TelegramInitDataMaxAge: time.Duration(maxAgeSec) * time.Second,
		CORSAllowedOrigins:     parseAllowedOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost,http://127.0.0.1")),
		RateLimitPerMinute:     rateLimit,
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://gardener:gardener@postgres:5432/gardener?sslmode=disable"),
		UploadDir:              getEnv("UPLOAD_DIR", "/data/uploads"),
		DataDir:                getEnv("DATA_DIR", "/app/data"),
		PythonBaseURL:          getEnv("PYTHON_BASE_URL", "http://classifier:5000"),
		AdminUser:              getEnv("ADMIN_USER", "admin"),
		AdminPassword:          getEnv("ADMIN_PASSWORD", ""),
		AdminSecret:            getEnv("ADMIN_SECRET", ""),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// sendToClassifier sends image to Python classification service
func sendToClassifier(imageData []byte, cropID string) (*ClassificationResult, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "upload.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}

	_, err = part.Write(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to write image  %v", err)
	}
	_ = writer.WriteField("crop_id", cropID)

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close writer: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", config.PythonServiceURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to classifier: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read classifier response: %v", err)
	}

	var result ClassificationResult
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse classifier response: %v", err)
	}

	return &result, nil
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

// generateRecommendation generates care recommendations using LLM
func generateRecommendation(classification *ClassificationResult, cropID string) (string, error) {
	prompts := promptsForCrop(cropID)
	prompt := fmt.Sprintf(`%s
Based on the following classification result from an image analysis, provide detailed care recommendations.

Classification Result:
- Detected: %s
- Confidence: %.2f%%
- Top predictions: %v

Please provide:
1. A brief explanation of what was detected
2. Specific care recommendations for this condition
3. Preventive measures if it's a disease
4. Treatment options if applicable
5. General tips for maintaining healthy trees

Respond in Russian language as the target audience is Russian-speaking gardeners.`,
		prompts.PhotoUserIntro,
		classification.Prediction,
		classification.Confidence*100,
		classification.TopPredictions,
	)

	if config.LLMAPIKey == "" {
		return generateTemplateRecommendation(classification), nil
	}
	return callLLMCompletion([]Message{
		{Role: "system", Content: prompts.PhotoSystem},
		{Role: "user", Content: prompt},
	})
}

// generateRecommendationWithHistory — рекомендации по фото с учётом предыдущих реплик диалога.
func generateRecommendationWithHistory(classification *ClassificationResult, caption string, history []Message, cropID string) (string, error) {
	prompts := promptsForCrop(cropID)
	prompt := fmt.Sprintf(`%s
Based on the following classification result from an image analysis, provide detailed care recommendations.

Classification Result:
- Detected: %s
- Confidence: %.2f%%
- Top predictions: %v

User caption (may be empty): %s

Please provide:
1. A brief explanation of what was detected
2. Specific care recommendations for this condition
3. Preventive measures if it's a disease
4. Treatment options if applicable
5. General tips for maintaining healthy trees

Respond in Russian language as the target audience is Russian-speaking gardeners.`,
		prompts.PhotoUserIntro,
		classification.Prediction,
		classification.Confidence*100,
		classification.TopPredictions,
		caption,
	)
	if config.LLMAPIKey == "" {
		return generateTemplateRecommendation(classification), nil
	}
	msgs := make([]Message, 0, len(history)+2)
	msgs = append(msgs, Message{Role: "system", Content: prompts.PhotoSystem})
	msgs = append(msgs, history...)
	msgs = append(msgs, Message{Role: "user", Content: prompt})
	return callLLMCompletion(msgs)
}

// generateTemplateRecommendation generates a template recommendation when LLM is not available
func generateTemplateRecommendation(classification *ClassificationResult) string {
	recommendations := map[string]string{
		"healthy_apple": `🍎 Здоровое яблоко обнаружено!

Что выявлено:
Ваше яблоко выглядит здоровым, без видимых признаков заболеваний.

Рекомендации по уходу:
• Продолжайте регулярный полив (2-3 раза в неделю)
• Вносите органические удобрения каждые 4-6 недель
• Проводите профилактическую обрезку сухих ветвей
• Следите за появлением вредителей
• Собирайте урожай вовремя для лучшего качества`,

		"apple_scab": `🍂 Обнаружена парша яблони!

Что выявлено:
Парша - грибковое заболевание, поражающее листья и плоды яблони.

Рекомендации по уходу:
• Удалите и сожгите все поражённые листья и плоды
• Обработайте дерево фунгицидами (бордоская жидкость, медный купорос)
• Проведите обработку ранней весной до распускания почек
• Осенью уберите всю опавшую листву
• Прореживайте крону для лучшей вентиляции

Профилактика:
• Выбирайте устойчивые сорта
• Регулярно проводите профилактические обработки
• Поддерживайте чистоту приствольного круга`,

		"black_rot": `🖤 Обнаружена чёрная гниль!

Что выявлено:
Чёрная гниль - серьёзное грибковое заболевание плодов.

Рекомендации по уходу:
• Немедленно удалите все поражённые плоды
• Обработайте дерево фунгицидами
• Улучшите циркуляцию воздуха вокруг дерева
• Избегайте повреждения плодов при уходе
• Собирайте урожай аккуратно

Профилактика:
• Регулярная обработка фунгицидами в сезон
• Контроль влажности
• Своевременная уборка урожая`,

		"cedar_apple_rust": `🧡 Обнаружена кедрово-яблоневая ржавчина!

Что выявлено:
Грибковое заболевание, требующее наличия двух хозяев (яблоня и можжевельник).

Рекомендации по уходу:
• По возможности удалите nearby можжевельники
• Обработайте фунгицидами содержащими серу
• Удаляйте поражённые листья
• Проводите обработку весной до цветения

Профилактика:
• Сажайте яблони подальше от можжевельников
• Регулярный осмотр деревьев`,

		"powdery_mildew": `⚪ Обнаружена мучнистая роса!

Что выявлено:
Грибковое заболевание, проявляющееся белым налётом.

Рекомендации по уходу:
• Обработайте раствором соды (1 ст. ложка на 1 л воды)
• Используйте серные препараты
• Удалите сильно поражённые побеги
• Улучшите вентиляцию кроны

Профилактика:
• Не перекармливайте азотными удобрениями
• Соблюдайте режим полива
• Проводите профилактические обработки`,

		"fire_blight": `🔥 Обнаружен бактериальный ожог!

Что выявлено:
Серьёзное бактериальное заболевание, требующее немедленного вмешательства.

Рекомендации по уходу:
• Срочно удалите все поражённые ветви (на 20-30 см ниже поражения)
• Дезинфицируйте инструменты после каждой обрезки
• Обработайте антибиотиками для растений
• При сильном поражении может потребоваться удаление дерева

Профилактика:
• Контроль насекомых-опылителей
• Избегайте обрезки во влажную погоду
• Выбирайте устойчивые сорта`,

		"healthy_leaf": `🌿 Здоровый лист яблони!

Что выявлено:
Листья выглядят здоровыми, признаков заболеваний нет.

Рекомендации по уходу:
• Продолжайте текущий режим ухода
• Регулярно осматривайте дерево
• Поддерживайте оптимальный полив
• Вносите сбалансированные удобрения
• Проводите своевременную обрезку`,

		"default": `🍎 Результаты анализа яблони

Что выявлено:
На основе анализа изображения была определена категория: {{PREDICTION}}
Уверенность классификации: {{CONFIDENCE}}%

Общие рекомендации по уходу за яблоней:
• Регулярный полив (особенно в засушливый период)
• Сезонная обрезка для формирования кроны
• Внесение органических и минеральных удобрений
• Профилактическая обработка от вредителей и болезней
• Мульчирование приствольного круга
• Защита от солнечных ожогов зимой

Для более точных рекомендаций обратитесь к специалисту или загрузите новое изображение.`,
	}

	rec, exists := recommendations[classification.Prediction]
	if !exists {
		rec = recommendations["default"]
		// Replace placeholders
		rec = replacePlaceholder(rec, "{{PREDICTION}}", classification.Prediction)
		confStr := fmt.Sprintf("%.1f", classification.Confidence*100)
		rec = replacePlaceholder(rec, "{{CONFIDENCE}}", confStr)
	}

	return rec
}

func replacePlaceholder(str, placeholder, value string) string {
	result := ""
	for i := 0; i < len(str); i++ {
		if i+len(placeholder) <= len(str) && str[i:i+len(placeholder)] == placeholder {
			result += value
			i += len(placeholder) - 1
		} else {
			result += string(str[i])
		}
	}
	return result
}

// handleClassification handles the image classification endpoint
func handleClassification(c *gin.Context) {
	// Get uploaded file
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to get image file",
		})
		return
	}
	defer file.Close()

	// Validate file type
	if header.Size > 10*1024*1024 { // 10MB limit
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Image too large (max 10MB)",
		})
		return
	}

	// Read file content
	imageData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to read image",
		})
		return
	}

	log.Printf("Received image: %s (%d bytes)", header.Filename, len(imageData))

	cropID, err := normalizeCropID(c.PostForm("crop_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	classification, err := sendToClassifier(imageData, cropID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Classification failed: %v", err),
		})
		return
	}

	if !classification.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Classification error: %s", classification.Error),
		})
		return
	}

	// Generate recommendations
	recommendation, err := generateRecommendation(classification, cropID)
	if err != nil {
		log.Printf("Warning: Failed to generate LLM recommendation: %v", err)
		// Continue with template recommendation
		recommendation = generateTemplateRecommendation(classification)
	}

	// Prepare response
	response := RecommendationResponse{
		Success:              true,
		ClassificationResult: *classification,
		Recommendation:       recommendation,
	}

	c.JSON(http.StatusOK, response)
}

// handleHealthCheck handles health check endpoint
func handleHealthCheck(c *gin.Context) {
	payload := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}
	if chatStore != nil && chatStore.pool != nil {
		if err := chatStore.pool.Ping(c.Request.Context()); err != nil {
			payload["status"] = "degraded"
			payload["database"] = "unreachable"
		} else {
			payload["database"] = "ok"
		}
	}
	c.JSON(http.StatusOK, payload)
}

func main() {
	// Load configuration
	config = loadConfig()

	log.Printf("Starting Apple Gardener Server...")
	log.Printf("Python Classify URL: %s", config.PythonServiceURL)
	log.Printf("Python RAG context URL: %s", config.PythonRAGURL)
	log.Printf("LLM Model: %s", config.LLMModel)
	if config.LLMAPIKey != "" {
		log.Printf("LLM API Key: configured")
	} else {
		log.Printf("LLM API Key: not configured (using template responses)")
	}
	if config.TelegramAuthDisabled {
		log.Printf("Telegram auth: DISABLED (dev mode only)")
	} else if config.TelegramBotToken != "" {
		log.Printf("Telegram auth: enabled")
	} else {
		log.Printf("Telegram auth: WARNING — TELEGRAM_BOT_TOKEN not set, protected routes will reject clients")
	}
	log.Printf("CORS origins: %v", config.CORSAllowedOrigins)
	log.Printf("Rate limit: %d req/min per user", config.RateLimitPerMinute)
	log.Printf("Database URL: configured")
	log.Printf("Upload dir: %s", config.UploadDir)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := waitForPostgres(ctx, config.DatabaseURL, 30)
	if err != nil {
		log.Fatalf("PostgreSQL: %v", err)
	}
	migDir, err := findMigrationsDir()
	if err != nil {
		log.Fatalf("Migrations: %v", err)
	}
	if err := runAllMigrations(ctx, pool, migDir); err != nil {
		log.Fatalf("Apply migrations: %v", err)
	}
	pool.Close()

	if err := loadCropCatalog(); err != nil {
		log.Fatalf("Crops config: %v", err)
	}
	if err := loadPromptCatalog(); err != nil {
		log.Fatalf("Prompts config: %v", err)
	}
	if err := loadOnboardingConfig(); err != nil {
		log.Fatalf("Onboarding config: %v", err)
	}

	chatStore, err = newChatStore(context.Background(), config.DatabaseURL, config.UploadDir)
	if err != nil {
		log.Fatalf("ChatStore: %v", err)
	}
	defer chatStore.Close()
	log.Printf("PostgreSQL: connected, migrations from %s", migDir)
	log.Printf("Crops loaded: %d, default=%s", len(cropCatalog.Crops), cropCatalog.DefaultCrop)

	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create router
	router := gin.Default()
	router.Use(corsMiddleware(config.CORSAllowedOrigins))
	router.Use(func(c *gin.Context) {
		if !strings.Contains(c.Request.URL.Path, "/media/") {
			c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		c.Next()
	})

	rl := newRateLimiter(config.RateLimitPerMinute, time.Minute)

	// Публичные маршруты (без Telegram auth)
	router.GET("/health", handleHealthCheck)
	router.GET("/api/health", handleHealthCheck)
	router.GET("/crops", handleListCrops)
	router.GET("/api/crops", handleListCrops)
	router.GET("/onboarding", handleOnboarding)
	router.GET("/api/onboarding", handleOnboarding)

	registerAdminRoutes(router, config)

	// Защищённые маршруты (Telegram initData + rate limit)
	registerProtectedRoutes(router, config, rl)

	// Start server
	serverAddr := fmt.Sprintf(":%s", config.ServerPort)
	log.Printf("Server starting on port %s", config.ServerPort)

	if err := router.Run(serverAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
