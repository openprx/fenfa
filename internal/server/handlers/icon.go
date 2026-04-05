package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

// AppIcon serves the app icon as PNG under /icon/:appID.png
func AppIcon(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appID")
		var app store.App
		if err := db.Where("id = ?", appID).First(&app).Error; err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		serveIconPath(c, resolveStoredIconPath(app.IconPath, filepath.ToSlash(filepath.Join("uploads", app.ID, "icon.png"))))
	}
}

// ProductIcon serves the product icon under /icon/products/:productID.
func ProductIcon(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("productID")
		var product store.Product
		if err := db.Where("id = ?", productID).First(&product).Error; err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		serveIconPath(c, resolveStoredIconPath(product.IconPath, filepath.ToSlash(filepath.Join("uploads", product.ID, "icon.png"))))
	}
}

func resolveStoredIconPath(storedPath, fallbackPath string) string {
	if storedPath != "" {
		return storedPath
	}
	return fallbackPath
}

func serveIconPath(c *gin.Context, iconPath string) {
	if _, err := os.Stat(iconPath); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	b, err := os.ReadFile(iconPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ct := http.DetectContentType(b)
	if ct == "application/octet-stream" {
		ct = "image/png"
	}
	c.Data(http.StatusOK, ct, b)
}
