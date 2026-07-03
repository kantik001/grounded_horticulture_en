package main

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxUploadImageBytes = 10 * 1024 * 1024

// classifyAndRecommendResult holds CV output and recommendation text for one photo.
type classifyAndRecommendResult struct {
	Classification   *ClassificationResult
	Recommendation   string
	UsedLLMTemplate bool
}

// readUploadImage reads a multipart file with size check.
func readUploadImage(file multipart.File, size int64) ([]byte, error) {
	if size > maxUploadImageBytes {
		return nil, fmt.Errorf("image too large (max 10 MB)")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxUploadImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("could not read image: %w", err)
	}
	if len(data) > maxUploadImageBytes {
		return nil, fmt.Errorf("image too large (max 10 MB)")
	}
	return data, nil
}

// readImageFromFormFile reads multipart image field via readUploadImage.
func readImageFromFormFile(c *gin.Context, field string) ([]byte, error) {
	fh, err := c.FormFile(field)
	if err != nil || fh == nil {
		return nil, nil
	}
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("could not open image file")
	}
	defer f.Close()
	return readUploadImage(f, fh.Size)
}

// classifyAndRecommend runs Python CV then LLM recommendation or config template.
func classifyAndRecommend(ctx context.Context, imageData []byte, cropID, caption string, history []Message) (*classifyAndRecommendResult, error) {
	if err := requireCVEnabled(cropID); err != nil {
		return nil, err
	}
	classification, err := sendToClassifier(ctx, imageData, cropID)
	if err != nil {
		return nil, fmt.Errorf("classification error: %w", err)
	}
	if classification == nil || !classification.Success {
		msg := "classification error"
		if classification != nil && classification.Error != "" {
			msg = classification.Error
		}
		return nil, fmt.Errorf("%s", msg)
	}

	recommendation, recErr := generatePhotoRecommendation(ctx, classification, caption, history, cropID)
	usedTemplate := config.LLMAPIKey == "" || recErr != nil
	if recErr != nil {
		recommendation = generateTemplateRecommendation(classification)
	}
	recommendation = appendPhotoBetaNotice(recommendation)

	return &classifyAndRecommendResult{
		Classification:    classification,
		Recommendation:    recommendation,
		UsedLLMTemplate:   usedTemplate,
	}, nil
}

// appendPhotoBetaNotice appends CV beta disclaimer from config/branding.json.
func appendPhotoBetaNotice(recommendation string) string {
	notice := strings.TrimSpace(currentCatalogs().Branding.PhotoBetaNotice)
	if notice == "" {
		return recommendation
	}
	body := strings.TrimSpace(recommendation)
	if body == "" {
		return notice
	}
	if strings.Contains(body, notice) {
		return body
	}
	return body + "\n\n" + notice
}

// parseClassifyForm reads image and crop_id from POST /classify.
func parseClassifyForm(c *gin.Context) (imageData []byte, cropID string, filename string, err error) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		return nil, "", "", fmt.Errorf("could not read image file field")
	}
	defer file.Close()

	imageData, err = readUploadImage(file, header.Size)
	if err != nil {
		return nil, "", "", err
	}

	cropID, err = normalizeCropID(c.PostForm("crop_id"))
	if err != nil {
		return nil, "", "", err
	}
	return imageData, cropID, header.Filename, nil
}
