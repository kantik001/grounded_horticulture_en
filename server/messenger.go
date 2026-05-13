package main

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxMessengerImageBytes = 10 * 1024 * 1024

// handleNewSession создаёт новую сессию переписки.
func handleNewSession(c *gin.Context) {
	id, s := createSession()
	_ = s
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": id,
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
	s := getSession(id)
	if s == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Сессия не найдена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": id,
		"messages":   s.snapshot(),
	})
}

type jsonMessageRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

// handleMessage — единая точка: текст (RAG+LLM) и/или фото (классификация+LLM).
func handleMessage(c *gin.Context) {
	ct := c.GetHeader("Content-Type")
	var sessionID string
	var text string
	var imageData []byte

	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(int64(maxMessengerImageBytes + 512*1024)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Некорректный multipart"})
			return
		}
		sessionID = strings.TrimSpace(c.PostForm("session_id"))
		text = strings.TrimSpace(c.PostForm("text"))
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
	}

	if text == "" && len(imageData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Нужен текст или изображение"})
		return
	}

	sid, sess, _ := getOrCreateSession(sessionID)

	if len(imageData) > 0 {
		handleImageMessage(c, sid, sess, text, imageData)
		return
	}
	handleTextMessage(c, sid, sess, text)
}

func handleTextMessage(c *gin.Context, sid string, sess *sessionData, text string) {
	prior := sess.historyForLLM(0)
	answer, ok, errMsg, ragSoft := answerWithRAG(text, prior)

	userMsg := ChatMessage{Role: "user", Content: text, Kind: "text"}
	sess.appendMessage(userMsg)

	if ragSoft {
		sess.appendMessage(ChatMessage{Role: "assistant", Content: errMsg, Kind: "assistant"})
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"session_id": sid,
			"messages":   sess.snapshot(),
			"error":      errMsg,
		})
		return
	}
	if !ok {
		if strings.Contains(errMsg, "LLM_API_KEY") {
			sess.appendMessage(ChatMessage{Role: "assistant", Content: errMsg, Kind: "assistant"})
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success":    false,
				"session_id": sid,
				"messages":   sess.snapshot(),
				"error":      errMsg,
			})
			return
		}
		sess.appendMessage(ChatMessage{Role: "assistant", Content: "Ошибка: " + errMsg, Kind: "assistant"})
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"session_id": sid,
			"messages":   sess.snapshot(),
			"error":      errMsg,
		})
		return
	}

	sess.appendMessage(ChatMessage{Role: "assistant", Content: answer, Kind: "assistant"})
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": sid,
		"messages":   sess.snapshot(),
	})
}

func handleImageMessage(c *gin.Context, sid string, sess *sessionData, caption string, imageData []byte) {
	prior := sess.historyForLLM(0)
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imageData)

	classification, err := sendToClassifier(imageData)
	if err != nil || classification == nil || !classification.Success {
		errText := "Не удалось классифицировать изображение"
		if err != nil {
			errText = err.Error()
		} else if classification != nil && classification.Error != "" {
			errText = classification.Error
		}
		log.Printf("Messenger classify error: %v", err)
		sess.appendMessage(ChatMessage{
			Role: "user", Content: caption, ImageDataURL: dataURL, Kind: "image",
		})
		sess.appendMessage(ChatMessage{Role: "assistant", Content: errText, Kind: "assistant"})
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"session_id": sid,
			"messages":   sess.snapshot(),
			"error":      errText,
		})
		return
	}

	userMsg := ChatMessage{
		Role:            "user",
		Content:         caption,
		ImageDataURL:    dataURL,
		ClassPrediction: classification.Prediction,
		ClassConfidence: classification.Confidence,
		Kind:            "image",
	}
	sess.appendMessage(userMsg)

	recommendation, err := generateRecommendationWithHistory(classification, caption, prior)
	if err != nil {
		log.Printf("Messenger LLM image error: %v", err)
		recommendation = generateTemplateRecommendation(classification)
	}

	sess.appendMessage(ChatMessage{Role: "assistant", Content: recommendation, Kind: "assistant"})
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": sid,
		"messages":   sess.snapshot(),
	})
}
