package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Healthz(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": gin.H{"code": "DB_DOWN", "message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"status": "ok"}})
	}
}

