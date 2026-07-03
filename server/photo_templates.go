package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// loadPhotoTemplates reads photo_templates.json into a prediction-to-text map.
func loadPhotoTemplates() (map[string]string, error) {
	path := photoTemplatesConfigPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read photo templates %s: %w", path, err)
	}
	var catalog map[string]string
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("parse photo templates: %w", err)
	}
	return catalog, nil
}

// photoTemplatesConfigPath resolves photo_templates.json via env override or known locations.
func photoTemplatesConfigPath() string {
	if p := os.Getenv("PHOTO_TEMPLATES_PATH"); p != "" {
		return p
	}
	for _, candidate := range []string{
		"/config/photo_templates.json",
		filepath.Join("..", "config", "photo_templates.json"),
		filepath.Join("config", "photo_templates.json"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("config", "photo_templates.json")
}
