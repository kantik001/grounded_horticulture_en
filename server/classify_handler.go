package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /classify: photo -> Python CV -> LLM recommendation or template (no chat session).
func handleClassification(c *gin.Context) {
	imageData, cropID, filename, err := parseClassifyForm(c)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err)
		return
	}

	log.Printf("Received image: %s (%d bytes)", filename, len(imageData))

	result, err := classifyAndRecommend(c.Request.Context(), imageData, cropID, "", nil)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err)
		return
	}
	if result.UsedLLMTemplate {
		log.Printf("Photo recommendation: used template (LLM unavailable or error)")
	}

	c.JSON(http.StatusOK, RecommendationResponse{
		Success:              true,
		ClassificationResult: *result.Classification,
		Recommendation:       result.Recommendation,
	})
}
