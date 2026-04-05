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

// isPlatformCompatible checks if the user agent is compatible with the app platform
func isPlatformCompatible(platform, userAgent string) bool {
	ua := strings.ToLower(userAgent)

	switch strings.ToLower(platform) {
	case "ios":
		// iOS devices: iPhone, iPad, iPod
		return strings.Contains(ua, "iphone") ||
			strings.Contains(ua, "ipad") ||
			strings.Contains(ua, "ipod")
	case "android":
		// Android devices
		return strings.Contains(ua, "android")
	default:
		// Unknown platform, allow access
		return true
	}
}

// AppDetailJSON returns app and recent releases
func AppDetailJSON(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appID")
		var app store.App
		if err := db.Where("id = ?", appID).First(&app).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "app not found"}})
			return
		}

		// Check if app is published (only for non-admin requests)
		if !app.Published {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "app not found"}})
			return
		}

		var rels []store.Release
		if err := db.Where("app_id = ?", app.ID).Order("created_at desc").Limit(10).Find(&rels).Error; err != nil {
			rels = []store.Release{}
		}
		// choose selected release by query r
		selID := c.Query("r")
		var selected *store.Release
		for i := range rels {
			if selID != "" && rels[i].ID == selID {
				selected = &rels[i]
				break
			}
		}
		if selected == nil && len(rels) > 0 {
			selected = &rels[0]
		}
		// Determine domains (prefer DB settings; fall back to config/request)
		primary := cfg.GetPrimaryDomain()
		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err == nil {
			if settings.PrimaryDomain != "" {
				primary = settings.PrimaryDomain
			}
		}
		// Secondary domains for distribution (use first if present)
		downloadBase := ""
		if settings.SecondaryDomains != "" {
			var secs []string
			_ = json.Unmarshal([]byte(settings.SecondaryDomains), &secs)
			if len(secs) > 0 && secs[0] != "" {
				downloadBase = secs[0]
			}
		}
		// Determine scheme
		scheme := c.GetHeader("X-Forwarded-Proto")
		if scheme == "" && c.Request.TLS != nil {
			scheme = "https"
		}
		if scheme == "" {
			scheme = "http"
		}
		// Normalize domain value to match request scheme and ensure protocol present
		normalize := func(raw string) string {
			r := strings.TrimSpace(raw)
			if r == "" {
				return scheme + "://" + c.Request.Host
			}
			// default/localhost should use current host
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
			// no scheme provided
			return scheme + "://" + r
		}
		primary = normalize(primary)
		if downloadBase == "" {
			downloadBase = primary
		} else {
			downloadBase = normalize(downloadBase)
		}
		urls := gin.H{}
		iconURL := ""
		if app.IconPath != "" {
			iconURL = primary + "/icon/" + app.ID
		}

		if selected != nil {
			manifestURL := primary + "/ios/" + selected.ID + "/manifest.plist"
			urls = gin.H{
				"download":     downloadBase + "/d/" + selected.ID,
				"ios_manifest": manifestURL,
				"ios_install":  "itms-services://?action=download-manifest&url=" + url.QueryEscape(manifestURL),
				"release_page": primary + "/apps/" + app.ID + "?r=" + selected.ID,
			}
		}
		// Batch-load provisioning profiles for iOS apps
		profileMap := make(map[string]store.ProvisioningProfile)
		if app.Platform == "ios" && len(rels) > 0 {
			relIDs := make([]string, len(rels))
			for i, r := range rels {
				relIDs[i] = r.ID
			}
			var profiles []store.ProvisioningProfile
			db.Where("release_id IN ?", relIDs).Find(&profiles)
			for _, p := range profiles {
				profileMap[p.ReleaseID] = p
			}
		}
		// thin release summaries with per-release URLs
		summer := make([]gin.H, 0, len(rels))
		for _, r := range rels {
			relManifestURL := primary + "/ios/" + r.ID + "/manifest.plist"
			relData := gin.H{
				"id": r.ID, "version": r.Version, "build": r.Build, "created_at": r.CreatedAt,
				"download_count": r.DownloadCount, "channel": r.Channel,
				"min_os": r.MinOS, "changelog": r.Changelog,
				"urls": gin.H{
					"download":     downloadBase + "/d/" + r.ID,
					"ios_manifest": relManifestURL,
					"ios_install":  "itms-services://?action=download-manifest&url=" + url.QueryEscape(relManifestURL),
					"release_page": primary + "/apps/" + app.ID + "?r=" + r.ID,
				},
			}
			if p, ok := profileMap[r.ID]; ok {
				relData["provisioning_profile"] = formatProvisioningProfile(p)
			}
			summer = append(summer, relData)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
			"app":      gin.H{"id": app.ID, "name": app.Name, "platform": app.Platform, "bundle_id": app.BundleID, "application_id": app.ApplicationID, "icon_url": iconURL},
			"releases": summer,
			"urls":     urls,
		}})
	}
}

// formatProvisioningProfile formats a ProvisioningProfile for API response
func formatProvisioningProfile(p store.ProvisioningProfile) gin.H {
	result := gin.H{
		"uuid":                   p.UUID,
		"name":                   p.Name,
		"team_id":                p.TeamID,
		"team_name":              p.TeamName,
		"app_id_name":            p.AppIDName,
		"app_id_prefix":          p.AppIDPrefix,
		"bundle_id":              p.BundleID,
		"platform":               p.Platform,
		"profile_type":           p.ProfileType,
		"provisions_all_devices": p.ProvisionsAllDevices,
		"creation_date":          p.CreationDate,
		"expiration_date":        p.ExpirationDate,
	}

	// Parse certificates JSON
	if p.Certificates != "" {
		var certs []interface{}
		if err := json.Unmarshal([]byte(p.Certificates), &certs); err == nil {
			result["certificates"] = certs
		}
	}

	// Parse provisioned devices JSON
	if p.ProvisionedDevices != "" {
		var devices []string
		if err := json.Unmarshal([]byte(p.ProvisionedDevices), &devices); err == nil {
			result["provisioned_devices"] = devices
			result["device_count"] = len(devices)
		}
	}

	// Parse entitlements JSON
	if p.Entitlements != "" {
		var ent map[string]interface{}
		if err := json.Unmarshal([]byte(p.Entitlements), &ent); err == nil {
			result["entitlements"] = ent
		}
	}

	return result
}
