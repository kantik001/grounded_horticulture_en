package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// BrandingConfig holds Web App copy (domain pack, not core).
type BrandingConfig struct {
	AppTitle         string `json:"app_title"`
	HeaderEmoji      string `json:"header_emoji"`
	HeaderSubtitle   string `json:"header_subtitle"`
	CropLabel        string `json:"crop_label"`
	OnboardingTitle  string `json:"onboarding_title"`
	ChatDivider      string `json:"chat_divider"`
	Disclaimer       string `json:"disclaimer"`
	PhotoBetaNotice  string `json:"photo_beta_notice"`
}

// loadBrandingConfig reads branding.json into BrandingConfig.
func loadBrandingConfig() (BrandingConfig, error) {
	var branding BrandingConfig
	path := brandingConfigPath()
	body, err := os.ReadFile(path)
	if err != nil {
		return branding, err
	}
	if err := json.Unmarshal(body, &branding); err != nil {
		return branding, err
	}
	return branding, nil
}

// brandingConfigPath resolves branding.json via env override or known locations.
func brandingConfigPath() string {
	if p := os.Getenv("BRANDING_CONFIG_PATH"); p != "" {
		return p
	}
	for _, candidate := range []string{
		"/config/branding.json",
		filepath.Join("..", "config", "branding.json"),
		filepath.Join("config", "branding.json"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("config", "branding.json")
}

// GET /branding returns public Web App UI copy.
func handleBranding(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"branding": currentCatalogs().Branding,
	})
}
