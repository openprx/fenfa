package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/openprx/fenfa/internal/config"
)

// RequireToken checks X-Auth-Token header against config tokens by scope
func RequireToken(cfg *config.Config, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Auth-Token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error": gin.H{"code": "UNAUTHORIZED", "message": "missing token"}})
			return
		}
		if !allowedScope(cfg, scope, token) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": gin.H{"code": "FORBIDDEN", "message": "invalid token"}})
			return
		}
		c.Next()
	}
}

func allowedScope(cfg *config.Config, scope, token string) bool {
	if token == "" {
		return false
	}
	// Admin scope: only admin tokens
	if scope == "admin" {
		for _, t := range cfg.Auth.AdminTokens {
			if t == token {
				return true
			}
		}
		return false
	}
	// Other scopes (e.g., upload): allow either upload tokens or admin tokens
	for _, t := range cfg.Auth.UploadTokens {
		if t == token {
			return true
		}
	}
	for _, t := range cfg.Auth.AdminTokens {
		if t == token {
			return true
		}
	}
	return false
}
