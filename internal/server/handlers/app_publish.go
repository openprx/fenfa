package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

// PublishApp sets the app's published status to true
func PublishApp(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appID")

		var app store.App
		if err := db.Where("id = ?", appID).First(&app).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"ok": false,
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "App not found",
					},
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "DATABASE_ERROR",
					"message": "Failed to query app",
				},
			})
			return
		}

		// Update published status
		if err := db.Model(&app).Update("published", true).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "DATABASE_ERROR",
					"message": "Failed to update app",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"data": gin.H{
				"id":        app.ID,
				"published": true,
			},
		})
	}
}

// UnpublishApp sets the app's published status to false
func UnpublishApp(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appID")

		var app store.App
		if err := db.Where("id = ?", appID).First(&app).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"ok": false,
					"error": gin.H{
						"code":    "NOT_FOUND",
						"message": "App not found",
					},
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "DATABASE_ERROR",
					"message": "Failed to query app",
				},
			})
			return
		}

		// Update published status
		if err := db.Model(&app).Update("published", false).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "DATABASE_ERROR",
					"message": "Failed to update app",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"data": gin.H{
				"id":        app.ID,
				"published": false,
			},
		})
	}
}

