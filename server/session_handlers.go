package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var chatStore *ChatStore

type newSessionRequest struct {
	CropID string `json:"crop_id"`
}

// handleNewSession creates a new chat session for the current Telegram user.
func handleNewSession(c *gin.Context) {
	tgUser, err := ctxTelegramUser(c)
	if err != nil {
		jsonError(c, http.StatusUnauthorized, err)
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
			jsonError(c, http.StatusBadRequest, err)
			return
		}
	}

	ctx := c.Request.Context()
	userID, err := chatStore.UpsertUser(ctx, tgUser)
	if err != nil {
		log.Printf("UpsertUser: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
		return
	}
	sid, err := chatStore.CreateSession(ctx, userID, cropID)
	if err != nil {
		log.Printf("CreateSession: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not create session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": sid,
		"crop_id":    cropID,
		"messages":   []ChatMessage{},
	})
}

// handleHistory returns saved history for session_id.
func handleHistory(c *gin.Context) {
	id := strings.TrimSpace(c.Query("session_id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "session_id parameter required"})
		return
	}
	tgUser, err := ctxTelegramUser(c)
	if err != nil {
		jsonError(c, http.StatusUnauthorized, err)
		return
	}
	msgs, err := chatStore.ListMessages(c.Request.Context(), id, tgUser.ID)
	if err != nil {
		if err == errSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Session not found"})
			return
		}
		log.Printf("ListMessages: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
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

// handleMedia serves a photo by token (session owner only).
func handleMedia(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	tgUser, err := ctxTelegramUser(c)
	if err != nil {
		jsonError(c, http.StatusUnauthorized, err)
		return
	}
	ok, err := chatStore.UserCanAccessImage(c.Request.Context(), token, tgUser.ID)
	if err != nil || !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "File not found"})
		return
	}
	data, err := chatStore.ReadImage(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "File not found"})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", data)
}
