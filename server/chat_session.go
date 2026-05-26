package main

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// ChatMessage — одно сообщение в истории (для UI и восстановления контекста LLM).
type ChatMessage struct {
	ID              int64   `json:"id,omitempty"`
	Role            string  `json:"role"`
	Content         string  `json:"content"`
	ImageDataURL    string  `json:"image_data_url,omitempty"` // legacy
	ImageURL        string  `json:"image_url,omitempty"`
	ImageToken      string  `json:"-"`
	ClassPrediction string  `json:"class_prediction,omitempty"`
	ClassConfidence float64 `json:"class_confidence,omitempty"`
	Kind            string  `json:"kind,omitempty"`
	FeedbackRating  *int    `json:"feedback_rating,omitempty"` // 1 или -1, если пользователь оценил
}

func trimHistoryMessages(msgs []Message, max int) []Message {
	if len(msgs) <= max {
		return msgs
	}
	return msgs[len(msgs)-max:]
}

func (m ChatMessage) toLLMMessage() (Message, bool) {
	switch m.Role {
	case "assistant":
		if m.Content == "" {
			return Message{}, false
		}
		return Message{Role: "assistant", Content: m.Content}, true
	case "user":
		if m.ImageURL != "" || m.ImageDataURL != "" || m.ImageToken != "" {
			s := "Пользователь отправил фотографию яблони или листа."
			if m.ClassPrediction != "" {
				s += " Результат классификации модели: " + m.ClassPrediction + "."
				if m.ClassConfidence > 0 && m.ClassConfidence <= 1 {
					s += fmt.Sprintf(" Уверенность модели: %.0f%%.", m.ClassConfidence*100)
				}
			}
			if t := trimUserCaption(m.Content); t != "" {
				s += " Подпись к фото: " + t
			}
			return Message{Role: "user", Content: s}, true
		}
		if t := trimUserCaption(m.Content); t != "" {
			return Message{Role: "user", Content: t}, true
		}
		return Message{}, false
	default:
		return Message{}, false
	}
}

func trimUserCaption(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " "))
}

func ctxTelegramUser(c *gin.Context) (*TelegramUser, error) {
	if raw, ok := c.Get(ctxKeyTelegramUser); ok {
		if u, ok := raw.(*TelegramUser); ok && u != nil {
			return u, nil
		}
	}
	if raw, ok := c.Get(ctxKeyTelegramUserID); ok {
		if id, ok := raw.(int64); ok && id != 0 {
			return &TelegramUser{ID: id}, nil
		}
	}
	return nil, fmt.Errorf("telegram user not in context")
}
