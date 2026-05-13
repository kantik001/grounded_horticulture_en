package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

const maxSessionMessages = 80

// ChatMessage — одно сообщение в истории (для UI и восстановления контекста LLM).
type ChatMessage struct {
	Role            string  `json:"role"`
	Content         string  `json:"content"`
	ImageDataURL    string  `json:"image_data_url,omitempty"`
	ClassPrediction string  `json:"class_prediction,omitempty"`
	ClassConfidence float64 `json:"class_confidence,omitempty"`
	Kind            string  `json:"kind,omitempty"` // text | image | assistant
}

type sessionData struct {
	mu       sync.Mutex
	Messages []ChatMessage
}

var (
	sessionsMu sync.RWMutex
	sessions   = map[string]*sessionData{}
)

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func createSession() (string, *sessionData) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	id := newSessionID()
	s := &sessionData{Messages: make([]ChatMessage, 0, 8)}
	sessions[id] = s
	return id, s
}

func getSession(id string) *sessionData {
	if id == "" {
		return nil
	}
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	return sessions[id]
}

func getOrCreateSession(id string) (sid string, s *sessionData, created bool) {
	if id != "" {
		if s = getSession(id); s != nil {
			return id, s, false
		}
	}
	sid, s = createSession()
	return sid, s, true
}

func (s *sessionData) snapshot() []ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ChatMessage, len(s.Messages))
	copy(out, s.Messages)
	return out
}

func (s *sessionData) appendMessage(m ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Messages) >= maxSessionMessages {
		// отбрасываем старые с начала
		s.Messages = append([]ChatMessage{}, s.Messages[len(s.Messages)-maxSessionMessages+1:]...)
	}
	s.Messages = append(s.Messages, m)
}

// historyForLLM — последние сообщения до текущего хода (исключая tail).
func (s *sessionData) historyForLLM(excludeLastN int) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.Messages) - excludeLastN
	if n < 0 {
		n = 0
	}
	var out []Message
	for _, m := range s.Messages[:n] {
		if msg, ok := m.toLLMMessage(); ok {
			out = append(out, msg)
		}
	}
	return trimHistoryMessages(out, 24)
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
		if m.ImageDataURL != "" {
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
