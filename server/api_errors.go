package main

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

// publicAPIError returns a safe client message; details go to the log.
func publicAPIError(err error) string {
	if err == nil {
		return "Server error"
	}
	s := strings.TrimSpace(err.Error())
	if s == "" {
		return "Server error"
	}

	// Already user-facing from normalizeCropID, classify_flow, crop guards.
	if strings.Contains(s, "crop") ||
		strings.Contains(s, "image") ||
		strings.Contains(s, "file") ||
		strings.Contains(s, "session") ||
		strings.Contains(s, "assistant") ||
		strings.Contains(s, "recognition") ||
		strings.Contains(s, "Empty question") ||
		strings.Contains(s, "LLM_API_KEY") ||
		strings.Contains(s, "classification") ||
		strings.Contains(s, "unknown crop") {
		return s
	}

	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "timeout"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "rag request failed"),
		strings.Contains(lower, "classifier"):
		return "Analysis service is temporarily unavailable. Please try again later."
	case strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "telegram"):
		return "Authorization error. Open the app from the Telegram bot."
	default:
		log.Printf("publicAPIError (detail hidden): %v", err)
		return "Server error"
	}
}

func jsonError(c *gin.Context, code int, err error) {
	if err != nil {
		log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	}
	c.JSON(code, gin.H{"success": false, "error": publicAPIError(err)})
}
