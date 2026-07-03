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

// loadCropCatalog parses config/crops.json into a fresh cropsFile.
func loadCropCatalog() (cropsFile, error) {
	var catalog cropsFile
	path := cropsConfigPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return catalog, fmt.Errorf("read crops config %s: %w", path, err)
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		return catalog, fmt.Errorf("parse crops config: %w", err)
	}
	for id, c := range catalog.Crops {
		c.ID = id
		catalog.Crops[id] = c
	}
	if catalog.DefaultCrop == "" {
		catalog.DefaultCrop = "apple"
	}
	return catalog, nil
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
	catalog := currentCatalogs().Crops
	id := strings.TrimSpace(strings.ToLower(raw))
	if id == "" {
		id = catalog.DefaultCrop
	}
	if _, ok := catalog.Crops[id]; !ok {
		return "", fmt.Errorf("unknown crop: %s", raw)
	}
	return id, nil
}

// defaultCropID returns the default crop from the catalog.
func defaultCropID() string {
	if dc := currentCatalogs().Crops.DefaultCrop; dc != "" {
		return dc
	}
	return "apple"
}

// cropInfo returns crop metadata by id.
func cropInfo(cropID string) (CropInfo, bool) {
	c, ok := currentCatalogs().Crops.Crops[cropID]
	return c, ok
}

type cropPrompts struct {
	RAGSystem      string `json:"rag_system"`
	RAGTaskIntro   string `json:"rag_task_intro"`
	PhotoSystem    string `json:"photo_system"`
	PhotoUserIntro string `json:"photo_user_intro"`
	PhotoUserBody  string `json:"photo_user_body"`
}

// loadPromptCatalog parses config/prompts.json (RAG and photo system prompts).
func loadPromptCatalog() (map[string]cropPrompts, error) {
	path := promptsConfigPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read prompts config %s: %w", path, err)
	}
	var catalog map[string]cropPrompts
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("parse prompts config: %w", err)
	}
	return catalog, nil
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
	catalog := currentCatalogs().Prompts
	if p, ok := catalog[cropID]; ok {
		return p
	}
	if p, ok := catalog[defaultCropID()]; ok {
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
	catalog := currentCatalogs().Crops
	list := make([]gin.H, 0, len(catalog.Crops))
	for id, cinfo := range catalog.Crops {
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
		"default_crop": catalog.DefaultCrop,
		"crops":        list,
	})
}
