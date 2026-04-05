package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

func Download(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID := c.Param("releaseID")
		var rel store.Release
		if err := db.Where("id = ?", releaseID).First(&rel).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "release not found"}})
			return
		}

		ctx, err := loadReleaseVariantContext(db, rel)
		if err == nil {
			if !ctx.Product.Published || !ctx.Variant.Published {
				c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "variant not published"}})
				return
			}
		} else if rel.AppID != "" {
			var app store.App
			if err := db.Where("id = ?", rel.AppID).First(&app).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "app not found"}})
				return
			}
			if !app.Published {
				c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "app not published"}})
				return
			}
		} else {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "variant not found"}})
			return
		}

		// S3 storage: redirect to public URL
		if strings.HasPrefix(rel.StoragePath, "s3://") {
			var settings store.SystemSettings
			db.Where("id = ?", "default").First(&settings)
			if settings.S3PublicURL != "" {
				s3Key := strings.TrimPrefix(rel.StoragePath, "s3://")
				redirectURL := strings.TrimRight(settings.S3PublicURL, "/") + "/" + s3Key
				_ = db.Model(&rel).UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
				_ = db.Create(&store.Event{Type: "download", AppID: rel.AppID, VariantID: rel.VariantID, ReleaseID: rel.ID, IP: c.ClientIP(), UA: c.Request.UserAgent()}).Error
				c.Redirect(http.StatusFound, redirectURL)
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "S3 public URL not configured"}})
			return
		}

		// Local storage: serve file directly
		if _, err := os.Stat(rel.StoragePath); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "file missing"}})
			return
		}

		// Set safe headers for binary app packages
		ext := rel.FileExt
		if ext == "" {
			ext = detectFileExt(rel.StoragePath)
		}
		ctype := contentTypeForExt(ext)
		c.Header("Content-Type", ctype)
		c.Header("Content-Disposition", "attachment; filename=\""+filepath.Base(rel.StoragePath)+"\"")
		c.Header("Cache-Control", "no-transform")
		_ = db.Model(&rel).UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
		_ = db.Create(&store.Event{Type: "download", AppID: rel.AppID, VariantID: rel.VariantID, ReleaseID: rel.ID, IP: c.ClientIP(), UA: c.Request.UserAgent()}).Error

		c.File(rel.StoragePath) // http.ServeFile supports Range
	}
}
