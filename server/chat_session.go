package main

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// ChatMessage is one history entry (for UI and LLM context restoration).
type ChatMessage struct {
	ID              int64   `json:"id,omitempty"`
	Role            string  `json:"role"`
	Content         string  `json:"content"`
	ImageDataURL    string  `json:"image_data_url,omitempty"` // deprecated; see image_url
	ImageURL        string  `json:"image_url,omitempty"`
	ImageToken      string  `json:"-"`
	ClassPrediction string  `json:"class_prediction,omitempty"`
	ClassConfidence float64 `json:"class_confidence,omitempty"`
	Kind            string  `json:"kind,omitempty"`
	FeedbackRating  *int    `json:"feedback_rating,omitempty"` // 1 or -1 when user rated the reply
}

// trimHistoryMessages keeps the last max messages for LLM context.
func trimHistoryMessages(msgs []Message, max int) []Message {
	if len(msgs) <= max {
		return msgs
	}
	return msgs[len(msgs)-max:]
}

// toLLMMessage converts ChatMessage to the LLM API message format.
func (m ChatMessage) toLLMMessage() (Message, bool) {
	switch m.Role {
	case "assistant":
		if m.Content == "" {
			return Message{}, false
		}
		return Message{Role: "assistant", Content: m.Content}, true
	case "user":
		if m.ImageURL != "" || m.ImageDataURL != "" || m.ImageToken != "" {
			s := "The user sent a photo of an apple or leaf."
			if m.ClassPrediction != "" {
				s += " Model classification result: " + m.ClassPrediction + "."
				if m.ClassConfidence > 0 && m.ClassConfidence <= 1 {
					s += fmt.Sprintf(" Model confidence: %.0f%%.", m.ClassConfidence*100)
				}
			}
			if t := trimUserCaption(m.Content); t != "" {
				s += " Photo caption: " + t
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

// trimUserCaption normalizes the user caption (whitespace, line breaks).
func trimUserCaption(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " "))
}

// ctxTelegramUser returns TelegramUser from Gin context after auth middleware.
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
