package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// Entry point: Postgres, migrations, crop catalogs, Gin router, and HTTP server.
func main() {
	config = loadConfig()
	logStartup(config)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := waitForPostgres(ctx, config.DatabaseURL, 30)
	if err != nil {
		log.Fatalf("PostgreSQL: %v", err)
	}
	migDir, err := findMigrationsDir()
	if err != nil {
		log.Fatalf("Migrations: %v", err)
	}
	if err := runAllMigrations(ctx, pool, migDir); err != nil {
		log.Fatalf("Apply migrations: %v", err)
	}
	pool.Close()

	if err := loadRuntimeCatalogs(); err != nil {
		log.Fatalf("Runtime configs: %v", err)
	}
	loadAPIKeys(config)

	chatStore, err = newChatStore(context.Background(), config.DatabaseURL, config.UploadDir)
	if err != nil {
		log.Fatalf("ChatStore: %v", err)
	}
	defer chatStore.Close()
	log.Printf("PostgreSQL: connected, migrations from %s", migDir)
	crops := currentCatalogs().Crops
	log.Printf("Crops loaded: %d, default=%s", len(crops.Crops), crops.DefaultCrop)

	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.Use(corsMiddleware(config.CORSAllowedOrigins))
	router.Use(metricsMiddleware())
	router.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.Contains(path, "/media/") || strings.HasSuffix(path, "/stream") || path == "/metrics" {
			c.Next()
			return
		}
		c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		c.Next()
	})

	rl := newRateLimiter(config.RateLimitPerMinute, time.Minute)

	registerPublicRoutes(router)
	registerAdminRoutes(router, config)
	registerProtectedRoutes(router, config, rl)
	startConfigReloadWatcher()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", config.ServerPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Server starting on port %s", config.ServerPort)
		errCh <- srv.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("Failed to start server: %v", err)
	case sig := <-stopCh:
		log.Printf("Received %s, shutting down gracefully (max %s)", sig, shutdownTimeout)
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown incomplete: %v", err)
	} else {
		log.Printf("Server stopped cleanly")
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Server error: %v", err)
	}
}

// shutdownTimeout bounds in-flight request draining on SIGINT/SIGTERM.
// LLM streaming can take tens of seconds; give it room without hanging deploys.
const shutdownTimeout = 30 * time.Second
