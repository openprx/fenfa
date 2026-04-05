package handlers

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
)

func IOSManifest(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID := c.Param("releaseID")
		var rel store.Release
		if err := db.Where("id = ?", releaseID).First(&rel).Error; err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		ctx, err := loadReleaseVariantContext(db, rel)
		if err != nil {
			if rel.AppID == "" {
				c.String(http.StatusNotFound, "variant not found")
				return
			}
			var app store.App
			if err := db.Where("id = ?", rel.AppID).First(&app).Error; err != nil {
				c.String(http.StatusNotFound, "app not found")
				return
			}
			if !app.Published {
				c.String(http.StatusNotFound, "app not published")
				return
			}
			ctx.Variant = store.Variant{
				Platform:   app.Platform,
				Identifier: app.BundleID,
			}
			ctx.Product = store.Product{
				Name:      app.Name,
				Published: app.Published,
			}
		}
		if !ctx.Product.Published || !ctx.Variant.Published && ctx.Variant.ID != "" {
			c.String(http.StatusNotFound, "variant not published")
			return
		}
		if !strings.EqualFold(ctx.Variant.Platform, "ios") {
			c.String(http.StatusNotFound, "not an ios release")
			return
		}

		// iOS manifest should only be accessible from iOS devices
		ua := c.Request.UserAgent()
		if !isPlatformCompatible("ios", ua) {
			c.String(http.StatusForbidden, "This app is only available on iOS devices")
			return
		}

		// Get primary domain from database settings, fallback to config
		base := cfg.GetPrimaryDomain()
		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err == nil {
			if settings.PrimaryDomain != "" {
				base = settings.PrimaryDomain
			}
		}

		dl := fmt.Sprintf("%s/d/%s", base, rel.ID)

		ver := rel.Version
		if rel.Build > 0 && ver != "" {
			ver = fmt.Sprintf("%s (%d)", rel.Version, rel.Build)
		}
		c.Header("Content-Type", "application/xml")
		c.String(http.StatusOK, plistTemplate(dl, ctx.Variant.Identifier, ver, ctx.Product.Name))
	}
}

func plistTemplate(downloadURL, bundleID, version, title string) string {
	esc := func(s string) string { return html.EscapeString(s) }
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>items</key>
  <array>
    <dict>
      <key>assets</key>
      <array>
        <dict>
          <key>kind</key><string>software-package</string>
          <key>url</key><string>%s</string>
        </dict>
      </array>
      <key>metadata</key>
      <dict>
        <key>bundle-identifier</key><string>%s</string>
        <key>bundle-version</key><string>%s</string>
        <key>kind</key><string>software</string>
        <key>title</key><string>%s</string>
      </dict>
    </dict>
  </array>
</dict>
</plist>`, esc(downloadURL), esc(bundleID), esc(version), esc(title))
}
