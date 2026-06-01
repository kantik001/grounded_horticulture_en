package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleClassification handles the image classification endpoint.
func handleClassification(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to get image file",
		})
		return
	}
	defer file.Close()

	if header.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Image too large (max 10MB)",
		})
		return
	}

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

	recommendation, err := generateRecommendation(classification, cropID)
	if err != nil {
		log.Printf("Warning: Failed to generate LLM recommendation: %v", err)
		recommendation = generateTemplateRecommendation(classification)
	}

	response := RecommendationResponse{
		Success:              true,
		ClassificationResult: *classification,
		Recommendation:       recommendation,
	}
	c.JSON(http.StatusOK, response)
}
