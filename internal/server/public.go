package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
	"gorm.io/gorm"
)

func handleProductPage(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")

		var product store.Product
		if err := db.Where("slug = ? AND published = ?", slug, true).First(&product).Error; err != nil {
			c.HTML(http.StatusNotFound, "app.html", gin.H{
				"Title": "Not Found",
			})
			return
		}

		var variants []store.Variant
		db.Where("product_id = ? AND published = ?", product.ID, true).
			Order("sort_order ASC, platform ASC").Find(&variants)

		type variantView struct {
			store.Variant
			LatestRelease *store.Release `json:"latest_release"`
		}

		views := make([]variantView, 0, len(variants))
		for _, v := range variants {
			vv := variantView{Variant: v}
			var rel store.Release
			if err := db.Where("variant_id = ?", v.ID).Order("build DESC, created_at DESC").First(&rel).Error; err == nil {
				vv.LatestRelease = &rel
			}
			views = append(views, vv)
		}

		c.HTML(http.StatusOK, "app.html", gin.H{
			"Title":      product.Name,
			"Product":    product,
			"Variants":   views,
			"BaseURL":    cfg.GetPrimaryDomain(),
			"StaticBase": "/static/front",
		})
	}
}

func handleDownload(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID := c.Param("releaseID")

		var release store.Release
		if err := db.Where("id = ?", releaseID).First(&release).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "release not found")
			return
		}

		// Log download event
		db.Create(&store.Event{
			Type:      "download",
			VariantID: release.VariantID,
			ReleaseID: release.ID,
			IP:        c.ClientIP(),
			UA:        c.GetHeader("User-Agent"),
		})

		// Increment download count
		db.Model(&release).Update("download_count", gorm.Expr("download_count + 1"))

		// Serve file
		fullPath := filepath.Join(cfg.Server.DataDir, release.StoragePath)
		if _, err := os.Stat(fullPath); err != nil {
			fail(c, http.StatusNotFound, "not_found", "file not found on disk")
			return
		}

		fileName := release.FileName
		if fileName == "" {
			fileName = "app." + release.FileExt
		}

		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
		c.File(fullPath)
	}
}

func handleManifest(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID := c.Param("releaseID")

		var release store.Release
		if err := db.Where("id = ?", releaseID).First(&release).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "release not found")
			return
		}

		var variant store.Variant
		db.Where("id = ?", release.VariantID).First(&variant)

		var product store.Product
		db.Where("id = ?", variant.ProductID).First(&product)

		domain := cfg.GetPrimaryDomain()
		downloadURL := fmt.Sprintf("%s/d/%s", domain, release.ID)
		iconURL := domain + product.IconPath

		bundleID := variant.Identifier
		if bundleID == "" {
			bundleID = "com.fenfa.app"
		}

		manifest := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>items</key>
	<array>
		<dict>
			<key>assets</key>
			<array>
				<dict>
					<key>kind</key>
					<string>software-package</string>
					<key>url</key>
					<string>%s</string>
				</dict>
				<dict>
					<key>kind</key>
					<string>display-image</string>
					<key>url</key>
					<string>%s</string>
				</dict>
			</array>
			<key>metadata</key>
			<dict>
				<key>bundle-identifier</key>
				<string>%s</string>
				<key>bundle-version</key>
				<string>%s</string>
				<key>kind</key>
				<string>software</string>
				<key>title</key>
				<string>%s</string>
			</dict>
		</dict>
	</array>
</dict>
</plist>`, downloadURL, iconURL, bundleID, release.Version, product.Name)

		c.Data(http.StatusOK, "text/xml; charset=utf-8", []byte(manifest))
	}
}
