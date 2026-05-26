package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ctxKeyTelegramUserID = "telegram_user_id"
	ctxKeyTelegramUser   = "telegram_user"
	headerTelegramInit   = "X-Telegram-Init-Data"
)

func parseAllowedOrigins(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// corsMiddleware разрешает запросы только с перечисленных Origin (безопаснее, чем *).
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowAll := len(allowedOrigins) == 0
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowAll {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else if _, ok := originSet[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
			c.Writer.Header().Set("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With, X-Telegram-Init-Data, Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// telegramAuthMiddleware проверяет подпись initData от Telegram Web App.
func telegramAuthMiddleware(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.TelegramAuthDisabled {
			devID := int64(0)
			if h := strings.TrimSpace(c.GetHeader("X-Dev-User-Id")); h != "" {
				if id, err := strconv.ParseInt(h, 10, 64); err == nil {
					devID = id
				}
			}
			if devID == 0 {
				devID = 1
			}
			c.Set(ctxKeyTelegramUserID, devID)
			c.Next()
			return
		}

		initData := strings.TrimSpace(c.GetHeader(headerTelegramInit))
		if initData == "" {
			// Authorization: tma <initData>
			auth := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "tma ") {
				initData = strings.TrimSpace(auth[4:])
			}
		}
		if initData == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Требуется авторизация Telegram (заголовок X-Telegram-Init-Data). Откройте приложение из бота.",
			})
			return
		}

		user, err := validateTelegramInitData(initData, cfg.TelegramBotToken, cfg.TelegramInitDataMaxAge)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Недействительная авторизация Telegram: " + err.Error(),
			})
			return
		}

		c.Set(ctxKeyTelegramUserID, user.ID)
		c.Set(ctxKeyTelegramUser, user)
		c.Next()
	}
}

// protected группа маршрутов: CORS уже применён глобально, здесь auth + rate limit.
func registerProtectedRoutes(router *gin.Engine, cfg *Config, rl *rateLimiter) {
	auth := telegramAuthMiddleware(cfg)
	lim := rateLimitMiddleware(rl)

	mount := func(r gin.IRoutes) {
		r.POST("/classify", auth, lim, handleClassification)
		r.POST("/chat", auth, lim, handleChat)
		r.POST("/session", auth, lim, handleNewSession)
		r.GET("/history", auth, lim, handleHistory)
		r.POST("/message", auth, lim, handleMessage)
		r.GET("/media/:token", auth, lim, handleMedia)
	}

	mount(router.Group(""))
	mount(router.Group("/api"))
}
