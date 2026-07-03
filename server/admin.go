package main

import (
	"crypto/subtle"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var safeFilename = regexp.MustCompile(`^[a-zA-Z0-9._-]+\.txt$`)

// Basic Auth for /admin routes (ADMIN_USER / ADMIN_PASSWORD).
func adminBasicAuth(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.AdminPassword == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "Admin is disabled: set ADMIN_PASSWORD in .env",
			})
			return
		}
		user, pass, ok := c.Request.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.AdminUser)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.AdminPassword)) == 1
		if !ok || !userOK || !passOK {
			c.Header("WWW-Authenticate", `Basic realm="Garden Admin"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}

// registerAdminRoutes registers admin routes: articles, upload, RAG reindex.
func registerAdminRoutes(router *gin.Engine, cfg *Config) {
	auth := adminBasicAuth(cfg)
	g := router.Group("/admin")
	g.Use(auth)
	g.GET("/status", handleAdminStatus)
	g.GET("/articles", handleAdminListArticles)
	g.GET("/feedback", handleAdminFeedback)
	g.POST("/upload", handleAdminUpload)
	g.POST("/reindex", handleAdminReindex)

	api := router.Group("/api/admin")
	api.Use(auth)
	api.GET("/status", handleAdminStatus)
	api.GET("/articles", handleAdminListArticles)
	api.GET("/feedback", handleAdminFeedback)
	api.POST("/upload", handleAdminUpload)
	api.POST("/reindex", handleAdminReindex)
}

// handleAdminStatus is GET /admin/status: data_dir and crop count.
func handleAdminStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data_dir": config.DataDir,
		"crops":    len(currentCatalogs().Crops.Crops),
	})
}

// handleAdminFeedback is GET /admin/feedback?rating=-1&limit=50 — answer ratings with Q&A text.
func handleAdminFeedback(c *gin.Context) {
	if chatStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	ratingFilter := 0
	if r := strings.TrimSpace(c.Query("rating")); r != "" {
		switch r {
		case "1", "-1":
			if _, err := fmt.Sscanf(r, "%d", &ratingFilter); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "rating: 1, -1, or empty"})
				return
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "rating: 1, -1, or empty"})
			return
		}
	}
	limit := 50
	if l := strings.TrimSpace(c.Query("limit")); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil || limit < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "limit: number 1–200"})
			return
		}
	}
	items, summary, err := chatStore.ListFeedbackReport(c.Request.Context(), ratingFilter, limit)
	if err != nil {
		log.Printf("Admin feedback list: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not load ratings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"summary": summary,
		"items":   items,
	})
}

// handleAdminListArticles is GET /admin/articles: .txt article list for crop_id.
func handleAdminListArticles(c *gin.Context) {
	cropID, err := normalizeCropID(c.Query("crop_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	dir := filepath.Join(config.DataDir, cropID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "crop_id": cropID, "files": []string{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".txt") {
			files = append(files, e.Name())
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "crop_id": cropID, "files": files})
}

// handleAdminUpload is POST /admin/upload: upload a .txt article to data/{crop_id}/.
func handleAdminUpload(c *gin.Context) {
	cropID, err := normalizeCropID(c.PostForm("crop_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": ".txt file required"})
		return
	}
	name := filepath.Base(fh.Filename)
	if !safeFilename.MatchString(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Filename: Latin letters, digits, .txt"})
		return
	}
	if fh.Size > 2*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Max file size 2 MB"})
		return
	}
	dir := filepath.Join(config.DataDir, cropID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	dst := filepath.Join(dir, name)
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(src, 2*1024*1024+1)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	log.Printf("Admin upload: %s -> %s", name, dst)
	c.JSON(http.StatusOK, gin.H{"success": true, "crop_id": cropID, "filename": name, "path": dst})
}

// handleAdminReindex is POST /admin/reindex: trigger Chroma reindex in Python.
func handleAdminReindex(c *gin.Context) {
	if err := triggerRAGReindex(); err != nil {
		log.Printf("Admin reindex: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "RAG reindex started"})
}

// triggerRAGReindex calls POST /admin/reindex on the Python service with X-Admin-Secret.
func triggerRAGReindex() error {
	if config.AdminSecret == "" {
		return fmt.Errorf("ADMIN_SECRET is not set")
	}
	url := strings.TrimRight(config.PythonBaseURL, "/") + "/admin/reindex"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Admin-Secret", config.AdminSecret)
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("python reindex HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
