package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// CropInfo describes one crop in the product catalog.
type CropInfo struct {
	ID         string `json:"-"`
	NameRU     string `json:"name_ru"`
	NameEN     string `json:"name_en"`
	Emoji      string `json:"emoji"`
	CVEnabled  bool   `json:"cv_enabled"`
	RAGEnabled bool   `json:"rag_enabled"`
	UIHidden   bool   `json:"ui_hidden,omitempty"`
}

type cropsFile struct {
	DefaultCrop string              `json:"default_crop"`
	Crops       map[string]CropInfo `json:"crops"`
}

var cropCatalog cropsFile

// loadCropCatalog loads config/crops.json into memory (cropCatalog).
func loadCropCatalog() error {
	path := cropsConfigPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read crops config %s: %w", path, err)
	}
	if err := json.Unmarshal(body, &cropCatalog); err != nil {
		return fmt.Errorf("parse crops config: %w", err)
	}
	for id, c := range cropCatalog.Crops {
		c.ID = id
		cropCatalog.Crops[id] = c
	}
	if cropCatalog.DefaultCrop == "" {
		cropCatalog.DefaultCrop = "apple"
	}
	return nil
}

// cropsConfigPath returns the path to crops.json (env or common locations).
func cropsConfigPath() string {
	if p := os.Getenv("CROPS_CONFIG_PATH"); p != "" {
		return p
	}
	for _, candidate := range []string{
		"/config/crops.json",
		filepath.Join("..", "config", "crops.json"),
		filepath.Join("config", "crops.json"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("config", "crops.json")
}

// normalizeCropID normalizes crop_id and checks it exists in the catalog.
func normalizeCropID(raw string) (string, error) {
	id := strings.TrimSpace(strings.ToLower(raw))
	if id == "" {
		id = cropCatalog.DefaultCrop
	}
	if _, ok := cropCatalog.Crops[id]; !ok {
		return "", fmt.Errorf("unknown crop: %s", raw)
	}
	return id, nil
}

// defaultCropID returns the default crop from the catalog.
func defaultCropID() string {
	if cropCatalog.DefaultCrop != "" {
		return cropCatalog.DefaultCrop
	}
	return "apple"
}

// cropInfo returns crop metadata by id.
func cropInfo(cropID string) (CropInfo, bool) {
	c, ok := cropCatalog.Crops[cropID]
	return c, ok
}

type cropPrompts struct {
	RAGSystem      string `json:"rag_system"`
	RAGTaskIntro   string `json:"rag_task_intro"`
	PhotoSystem    string `json:"photo_system"`
	PhotoUserIntro string `json:"photo_user_intro"`
	PhotoUserBody  string `json:"photo_user_body"`
}

var promptCatalog map[string]cropPrompts

// loadPromptCatalog loads config/prompts.json (RAG and photo system prompts).
func loadPromptCatalog() error {
	path := promptsConfigPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read prompts config %s: %w", path, err)
	}
	if err := json.Unmarshal(body, &promptCatalog); err != nil {
		return fmt.Errorf("parse prompts config: %w", err)
	}
	return nil
}

// promptsConfigPath returns the path to prompts.json.
func promptsConfigPath() string {
	if p := os.Getenv("PROMPTS_CONFIG_PATH"); p != "" {
		return p
	}
	for _, candidate := range []string{
		"/config/prompts.json",
		filepath.Join("..", "config", "prompts.json"),
		filepath.Join("config", "prompts.json"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("config", "prompts.json")
}

// promptsForCrop returns prompts for a crop; falls back to default_crop or built-in strings.
func promptsForCrop(cropID string) cropPrompts {
	if p, ok := promptCatalog[cropID]; ok {
		return p
	}
	if p, ok := promptCatalog[defaultCropID()]; ok {
		return p
	}
	return cropPrompts{
		RAGSystem:      "You are a knowledgeable agronomist. Respond in English.",
		RAGTaskIntro:   "You are a knowledgeable agronomist. Answer strictly from the provided context.",
		PhotoSystem:    "You are an experienced grower. Give practical advice in English.",
		PhotoUserIntro: "You are an experienced grower.",
		PhotoUserBody:  "Photo analysis result:\n- Class: %s\n- Confidence: %.2f%%\n- Top 3: %v\n\nGive care recommendations in English.",
	}
}

// handleListCrops is GET /crops: crop list with cv_enabled / rag_enabled flags.
func handleListCrops(c *gin.Context) {
	list := make([]gin.H, 0, len(cropCatalog.Crops))
	for id, cinfo := range cropCatalog.Crops {
		if cinfo.UIHidden {
			continue
		}
		nameEN := cinfo.NameEN
		if nameEN == "" {
			nameEN = cinfo.NameRU
		}
		list = append(list, gin.H{
			"id":          id,
			"name_ru":     cinfo.NameRU,
			"name_en":     nameEN,
			"emoji":       cinfo.Emoji,
			"cv_enabled":  cinfo.CVEnabled,
			"rag_enabled": cinfo.RAGEnabled,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"default_crop": cropCatalog.DefaultCrop,
		"crops":        list,
	})
}
