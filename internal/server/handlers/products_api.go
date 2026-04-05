package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
)

type publicDomains struct {
	Primary      string
	DownloadBase string
}

// ProductDetail returns the shared shell page for product distribution.
func ProductDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "app.html", gin.H{
			"Title":   "Modern distribution",
			"Message": "",
		})
	}
}

// ProductDetailJSON returns a product and all of its platform variants.
func ProductDetailJSON(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		product, err := findProductForRequest(db, c)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "product not found"}})
			return
		}
		if !product.Published {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "product not published"}})
			return
		}

		var variants []store.Variant
		if err := db.Where("product_id = ? AND published = ?", product.ID, true).Order("sort_order asc, created_at asc").Find(&variants).Error; err != nil {
			variants = nil
		}

		variantIDs := make([]string, 0, len(variants))
		for _, variant := range variants {
			variantIDs = append(variantIDs, variant.ID)
		}

		releaseMap := map[string][]store.Release{}
		if len(variantIDs) > 0 {
			var releases []store.Release
			if err := db.Where("variant_id IN ?", variantIDs).Order("created_at desc").Find(&releases).Error; err == nil {
				for _, release := range releases {
					releaseMap[release.VariantID] = append(releaseMap[release.VariantID], release)
				}
			}
		}

		profileMap := loadProvisioningProfilesByVariantReleases(db, releaseMap)
		domains := resolvePublicDomains(c, db, cfg)
		productKey := productPathKey(product)
		iconURL := productIconURL(product, domains.Primary)

		items := make([]gin.H, 0, len(variants))
		for _, variant := range variants {
			releases := releaseMap[variant.ID]
			releasePayloads := make([]gin.H, 0, len(releases))
			var latest gin.H
			for _, release := range releases {
				releaseData := releasePayload(productKey, variant.Platform, domains, release)
				if profile, ok := profileMap[release.ID]; ok {
					releaseData["provisioning_profile"] = formatProvisioningProfile(profile)
				}
				releasePayloads = append(releasePayloads, releaseData)
				if latest == nil {
					latest = releaseData
				}
			}

			item := gin.H{
				"id":             variant.ID,
				"platform":       variant.Platform,
				"identifier":     variant.Identifier,
				"legacy_app_id":  variant.LegacyAppID,
				"display_name":   variant.DisplayName,
				"arch":           variant.Arch,
				"installer_type": variant.InstallerType,
				"min_os":         variant.MinOS,
				"published":      variant.Published,
				"latest_release": latest,
				"releases":       releasePayloads,
			}
			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
			"product": gin.H{
				"id":        product.ID,
				"slug":      product.Slug,
				"name":      product.Name,
				"icon_url":  iconURL,
				"published": product.Published,
			},
			"variants": items,
		}})
	}
}

func findProductForRequest(db *gorm.DB, c *gin.Context) (store.Product, error) {
	productID := c.Param("productID")
	if productID == "" {
		productID = c.Param("slug")
	}

	var product store.Product
	if err := db.Where("id = ?", productID).First(&product).Error; err == nil {
		return product, nil
	}

	if productID != "" {
		if err := db.Where("slug = ?", productID).First(&product).Error; err == nil {
			return product, nil
		}
	}

	return store.Product{}, gorm.ErrRecordNotFound
}

func loadProvisioningProfilesByVariantReleases(db *gorm.DB, releaseMap map[string][]store.Release) map[string]store.ProvisioningProfile {
	profileMap := make(map[string]store.ProvisioningProfile)
	var releaseIDs []string
	for _, releases := range releaseMap {
		for _, release := range releases {
			releaseIDs = append(releaseIDs, release.ID)
		}
	}
	if len(releaseIDs) == 0 {
		return profileMap
	}

	var profiles []store.ProvisioningProfile
	db.Where("release_id IN ?", releaseIDs).Find(&profiles)
	for _, profile := range profiles {
		profileMap[profile.ReleaseID] = profile
	}
	return profileMap
}

func releasePayload(productKey, platform string, domains publicDomains, release store.Release) gin.H {
	data := gin.H{
		"id":             release.ID,
		"version":        release.Version,
		"build":          release.Build,
		"created_at":     release.CreatedAt,
		"download_count": release.DownloadCount,
		"channel":        release.Channel,
		"min_os":         release.MinOS,
		"changelog":      release.Changelog,
		"actions":        releaseActions(productKey, platform, domains, release.ID),
	}
	return data
}

func releaseActions(productKey, platform string, domains publicDomains, releaseID string) gin.H {
	actions := gin.H{
		"download":     domains.DownloadBase + "/d/" + releaseID,
		"release_page": domains.Primary + "/products/" + productKey + "?r=" + releaseID,
	}
	if strings.EqualFold(platform, "ios") {
		manifestURL := domains.Primary + "/ios/" + releaseID + "/manifest.plist"
		actions["ios_manifest"] = manifestURL
		actions["ios_install"] = "itms-services://?action=download-manifest&url=" + url.QueryEscape(manifestURL)
	}
	return actions
}

func resolvePublicDomains(c *gin.Context, db *gorm.DB, cfg *config.Config) publicDomains {
	primary := cfg.GetPrimaryDomain()
	var settings store.SystemSettings
	if err := db.Where("id = ?", "default").First(&settings).Error; err == nil && settings.PrimaryDomain != "" {
		primary = settings.PrimaryDomain
	}

	downloadBase := ""
	if settings.SecondaryDomains != "" {
		var secs []string
		_ = json.Unmarshal([]byte(settings.SecondaryDomains), &secs)
		if len(secs) > 0 && secs[0] != "" {
			downloadBase = secs[0]
		}
	}

	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" && c.Request.TLS != nil {
		scheme = "https"
	}
	if scheme == "" {
		scheme = "http"
	}

	normalize := func(raw string) string {
		r := strings.TrimSpace(raw)
		if r == "" {
			return scheme + "://" + c.Request.Host
		}
		if strings.Contains(r, "localhost") || strings.Contains(r, "127.0.0.1") {
			return scheme + "://" + c.Request.Host
		}
		if strings.HasPrefix(r, "http://") {
			if scheme == "https" {
				return "https://" + strings.TrimPrefix(r, "http://")
			}
			return r
		}
		if strings.HasPrefix(r, "https://") {
			return r
		}
		return scheme + "://" + r
	}

	primary = normalize(primary)
	if downloadBase == "" {
		downloadBase = primary
	} else {
		downloadBase = normalize(downloadBase)
	}

	return publicDomains{
		Primary:      primary,
		DownloadBase: downloadBase,
	}
}

func productPathKey(product store.Product) string {
	if product.Slug != "" {
		return product.Slug
	}
	return product.ID
}

func productIconURL(product store.Product, primary string) string {
	if product.IconPath != "" {
		return primary + "/icon/products/" + product.ID
	}
	if product.LegacyAppID != nil && *product.LegacyAppID != "" {
		return primary + "/icon/" + *product.LegacyAppID
	}
	return ""
}
