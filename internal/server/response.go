package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

func newID(prefix string) string {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "00000"
	}
	return prefix + hex.EncodeToString(buf)[:8]
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}

func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"ok": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
