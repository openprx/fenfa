package handlers

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

// AdminListApps returns all apps (basic fields), newest first
func AdminListApps(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := db.Model(&store.App{})
		// optional fuzzy search by name/bundle_id/application_id
		if s := c.Query("q"); s != "" {
			like := "%" + s + "%"
			q = q.Where("name LIKE ? OR bundle_id LIKE ? OR application_id LIKE ?", like, like, like)
		}
		limit := 200
		var apps []store.App
		if err := q.Order("created_at desc").Limit(limit).Find(&apps).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": []gin.H{}}})
			return
		}
		items := make([]gin.H, 0, len(apps))
		for _, a := range apps {
			items = append(items, gin.H{
				"id":             a.ID,
				"name":           a.Name,
				"platform":       a.Platform,
				"bundle_id":      a.BundleID,
				"application_id": a.ApplicationID,
				"published":      a.Published,
				"created_at":     a.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": items}})
	}
}

// AdminGetApp returns a single app with releases and provisioning profiles (for admin use)
func AdminGetApp(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appID")
		var app store.App
		if err := db.Where("id = ?", appID).First(&app).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "app not found"}})
			return
		}

		// Get releases
		var rels []store.Release
		if err := db.Where("app_id = ?", app.ID).Order("created_at desc").Limit(10).Find(&rels).Error; err != nil {
			rels = []store.Release{}
		}

		// Build release list with provisioning profiles
		releases := make([]gin.H, 0, len(rels))
		for _, r := range rels {
			relData := gin.H{
				"id":             r.ID,
				"version":        r.Version,
				"build":          r.Build,
				"created_at":     r.CreatedAt,
				"download_count": r.DownloadCount,
				"channel":        r.Channel,
				"min_os":         r.MinOS,
				"changelog":      r.Changelog,
			}
			// Include provisioning profile for iOS releases
			if app.Platform == "ios" {
				var profile store.ProvisioningProfile
				if err := db.Where("release_id = ?", r.ID).First(&profile).Error; err == nil {
					relData["provisioning_profile"] = formatProvisioningProfile(profile)
				}
			}
			releases = append(releases, relData)
		}

		// Sort by created_at desc
		sort.SliceStable(releases, func(i, j int) bool {
			return rels[i].CreatedAt.After(rels[j].CreatedAt)
		})

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"data": gin.H{
				"app": gin.H{
					"id":             app.ID,
					"name":           app.Name,
					"platform":       app.Platform,
					"bundle_id":      app.BundleID,
					"application_id": app.ApplicationID,
					"published":      app.Published,
					"created_at":     app.CreatedAt,
				},
				"releases": releases,
			},
		})
	}
}
