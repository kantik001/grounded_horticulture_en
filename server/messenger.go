package main

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxMessengerImageBytes = 10 * 1024 * 1024

var chatStore *ChatStore

type newSessionRequest struct {
	CropID string `json:"crop_id"`
}

// handleNewSession создаёт новую сессию переписки для текущего Telegram-пользователя.
func handleNewSession(c *gin.Context) {
	tgUser, err := ctxTelegramUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}

	cropID := defaultCropID()
	var req newSessionRequest
	if c.Request.ContentLength > 0 {
		_ = c.ShouldBindJSON(&req)
	}
	if req.CropID != "" {
		cropID, err = normalizeCropID(req.CropID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	}

	ctx := c.Request.Context()
	userID, err := chatStore.UpsertUser(ctx, tgUser)
	if err != nil {
		log.Printf("UpsertUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка базы данных"})
		return
	}
	sid, err := chatStore.CreateSession(ctx, userID, cropID)
	if err != nil {
		log.Printf("CreateSession: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Не удалось создать сессию"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": sid,
		"crop_id":    cropID,
		"messages":   []ChatMessage{},
	})
}

// handleHistory возвращает сохранённую историю по session_id.
func handleHistory(c *gin.Context) {
	id := strings.TrimSpace(c.Query("session_id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Нужен параметр session_id"})
		return
	}
	tgUser, err := ctxTelegramUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	msgs, err := chatStore.ListMessages(c.Request.Context(), id, tgUser.ID)
	if err != nil {
		if err == errSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Сессия не найдена"})
			return
		}
		log.Printf("ListMessages: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка базы данных"})
		return
	}
	cropID, _ := chatStore.SessionCropID(c.Request.Context(), id, tgUser.ID)
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": id,
		"crop_id":    cropID,
		"messages":   msgs,
	})
}

// handleMedia отдаёт фото по token (только владельцу сессии).
func handleMedia(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	tgUser, err := ctxTelegramUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	ok, err := chatStore.UserCanAccessImage(c.Request.Context(), token, tgUser.ID)
	if err != nil || !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Файл не найден"})
		return
	}
	data, err := chatStore.ReadImage(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Файл не найден"})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", data)
}

type jsonMessageRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	CropID    string `json:"crop_id"`
}

// handleMessage — единая точка: текст (RAG+LLM) и/или фото (классификация+LLM).
func handleMessage(c *gin.Context) {
	ct := c.GetHeader("Content-Type")
	var sessionID string
	var text string
	var cropIDRaw string
	var imageData []byte

	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(int64(maxMessengerImageBytes + 512*1024)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Некорректный multipart"})
			return
		}
		sessionID = strings.TrimSpace(c.PostForm("session_id"))
		text = strings.TrimSpace(c.PostForm("text"))
		cropIDRaw = strings.TrimSpace(c.PostForm("crop_id"))
		fh, err := c.FormFile("image")
		if err == nil && fh != nil {
			if fh.Size > maxMessengerImageBytes {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Изображение слишком большое (макс. 10 МБ)"})
				return
			}
			f, err := fh.Open()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Не удалось прочитать файл"})
				return
			}
			imageData, err = io.ReadAll(io.LimitReader(f, maxMessengerImageBytes+1))
			_ = f.Close()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Ошибка чтения изображения"})
				return
			}
			if len(imageData) > maxMessengerImageBytes {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Изображение слишком большое"})
				return
			}
		}
	} else {
		var req jsonMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Ожидается JSON: session_id, text"})
			return
		}
		sessionID = strings.TrimSpace(req.SessionID)
		text = strings.TrimSpace(req.Text)
		cropIDRaw = strings.TrimSpace(req.CropID)
	}

	if text == "" && len(imageData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Нужен текст или изображение"})
		return
	}

	tgUser, err := ctxTelegramUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}

	requestCropID, err := normalizeCropID(cropIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	sid, sessionCrop, err := chatStore.GetOrCreateSession(ctx, sessionID, tgUser, requestCropID)
	if err != nil {
		log.Printf("GetOrCreateSession: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка сессии"})
		return
	}

	if len(imageData) > 0 {
		handleImageMessage(c, sid, sessionCrop, tgUser.ID, text, imageData)
		return
	}
	handleTextMessage(c, sid, sessionCrop, tgUser.ID, text)
}

func respondWithMessages(c *gin.Context, sid, cropID string, telegramID int64, extra gin.H, status int) {
	msgs, err := chatStore.ListMessages(c.Request.Context(), sid, telegramID)
	if err != nil {
		log.Printf("ListMessages after reply: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка базы данных"})
		return
	}
	body := gin.H{"success": true, "session_id": sid, "crop_id": cropID, "messages": msgs}
	for k, v := range extra {
		body[k] = v
	}
	c.JSON(status, body)
}

func handleTextMessage(c *gin.Context, sid, cropID string, telegramID int64, text string) {
	ctx := c.Request.Context()
	prior, err := chatStore.HistoryForLLM(ctx, sid, telegramID, 0)
	if err != nil {
		log.Printf("HistoryForLLM: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка истории"})
		return
	}
	answer, ok, errMsg, ragSoft := answerWithRAG(text, cropID, prior)

	if err := chatStore.AppendMessage(ctx, sid, ChatMessage{Role: "user", Content: text, Kind: "text"}); err != nil {
		log.Printf("AppendMessage user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка сохранения"})
		return
	}

	if ragSoft {
		_ = chatStore.AppendMessage(ctx, sid, ChatMessage{Role: "assistant", Content: errMsg, Kind: "assistant"})
		respondWithMessages(c, sid, cropID, telegramID, gin.H{"error": errMsg}, http.StatusOK)
		return
	}
	if !ok {
		_ = chatStore.AppendMessage(ctx, sid, ChatMessage{Role: "assistant", Content: "Ошибка: " + errMsg, Kind: "assistant"})
		status := http.StatusInternalServerError
		if strings.Contains(errMsg, "LLM_API_KEY") {
			status = http.StatusServiceUnavailable
		}
		respondWithMessages(c, sid, cropID, telegramID, gin.H{"success": false, "error": errMsg}, status)
		return
	}

	if err := chatStore.AppendMessage(ctx, sid, ChatMessage{Role: "assistant", Content: answer, Kind: "assistant"}); err != nil {
		log.Printf("AppendMessage assistant: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка сохранения"})
		return
	}
	respondWithMessages(c, sid, cropID, telegramID, nil, http.StatusOK)
}

func handleImageMessage(c *gin.Context, sid, cropID string, telegramID int64, caption string, imageData []byte) {
	ctx := c.Request.Context()
	prior, err := chatStore.HistoryForLLM(ctx, sid, telegramID, 0)
	if err != nil {
		log.Printf("HistoryForLLM: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ошибка истории"})
		return
	}

	token, err := chatStore.SaveImage(imageData)
	if err != nil {
		log.Printf("SaveImage: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Не удалось сохранить фото"})
		return
	}

	classification, clsErr := sendToClassifier(imageData, cropID)
	if clsErr != nil || classification == nil || !classification.Success {
		errText := "Не удалось классифицировать изображение"
		if clsErr != nil {
			errText = clsErr.Error()
		} else if classification != nil && classification.Error != "" {
			errText = classification.Error
		}
		log.Printf("Messenger classify error: %v", clsErr)
		_ = chatStore.AppendMessage(ctx, sid, ChatMessage{
			Role: "user", Content: caption, Kind: "image", ImageToken: token,
		})
		_ = chatStore.AppendMessage(ctx, sid, ChatMessage{Role: "assistant", Content: errText, Kind: "assistant"})
		respondWithMessages(c, sid, cropID, telegramID, gin.H{"error": errText}, http.StatusOK)
		return
	}

	_ = chatStore.AppendMessage(ctx, sid, ChatMessage{
		Role:            "user",
		Content:         caption,
		Kind:            "image",
		ImageToken:      token,
		ClassPrediction: classification.Prediction,
		ClassConfidence: classification.Confidence,
	})

	recommendation, recErr := generateRecommendationWithHistory(classification, caption, prior, cropID)
	if recErr != nil {
		log.Printf("Messenger LLM image error: %v", recErr)
		recommendation = generateTemplateRecommendation(classification)
	}

	_ = chatStore.AppendMessage(ctx, sid, ChatMessage{Role: "assistant", Content: recommendation, Kind: "assistant"})
	respondWithMessages(c, sid, cropID, telegramID, nil, http.StatusOK)
}
