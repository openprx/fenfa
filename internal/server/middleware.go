package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/config"
)

// authMiddleware checks X-Auth-Token header against the configured tokens.
// scope must be "upload" or "admin".
func authMiddleware(cfg *config.Config, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Auth-Token")
		if token == "" {
			fail(c, http.StatusUnauthorized, "unauthorized", "missing X-Auth-Token header")
			c.Abort()
			return
		}

		var validTokens []string
		switch scope {
		case "admin":
			validTokens = cfg.Auth.AdminTokens
		case "upload":
			// upload tokens also accept admin tokens
			validTokens = append(cfg.Auth.UploadTokens, cfg.Auth.AdminTokens...)
		default:
			validTokens = cfg.Auth.AdminTokens
		}

		for _, t := range validTokens {
			if t == token {
				c.Next()
				return
			}
		}

		fail(c, http.StatusForbidden, "forbidden", "invalid token")
		c.Abort()
	}
}
