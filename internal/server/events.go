package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/store"
	"gorm.io/gorm"
)

func handleListEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Query("variant_id")
		eventType := c.Query("type")
		from := c.Query("from")
		to := c.Query("to")
		limit := queryInt(c, "limit", 50)
		offset := queryInt(c, "offset", 0)

		tx := db.Model(&store.Event{}).Order("ts DESC")

		if variantID != "" {
			tx = tx.Where("variant_id = ?", variantID)
		}
		if eventType != "" {
			tx = tx.Where("type = ?", eventType)
		}
		if from != "" {
			if t, err := time.Parse("2006-01-02", from); err == nil {
				tx = tx.Where("ts >= ?", t)
			}
		}
		if to != "" {
			if t, err := time.Parse("2006-01-02", to); err == nil {
				tx = tx.Where("ts < ?", t.AddDate(0, 0, 1))
			}
		}

		var total int64
		tx.Count(&total)

		var events []store.Event
		tx.Limit(limit).Offset(offset).Find(&events)

		ok(c, gin.H{"items": events, "total": total})
	}
}

func handleVariantStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Param("id")

		var variant store.Variant
		if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "variant not found")
			return
		}

		var product store.Product
		db.Where("id = ?", variant.ProductID).First(&product)

		var releases []store.Release
		db.Where("variant_id = ?", variantID).Order("build DESC, created_at DESC").Find(&releases)

		ok(c, gin.H{
			"product":  product,
			"variant":  variant,
			"releases": releases,
		})
	}
}

// Export handlers

func handleExportReleases(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Query("variant_id")
		from := c.Query("from")
		to := c.Query("to")

		tx := db.Model(&store.Release{}).Order("created_at DESC")
		if variantID != "" {
			tx = tx.Where("variant_id = ?", variantID)
		}
		if from != "" {
			if t, err := time.Parse("2006-01-02", from); err == nil {
				tx = tx.Where("created_at >= ?", t)
			}
		}
		if to != "" {
			if t, err := time.Parse("2006-01-02", to); err == nil {
				tx = tx.Where("created_at < ?", t.AddDate(0, 0, 1))
			}
		}

		var releases []store.Release
		tx.Find(&releases)

		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=releases.csv")

		var sb strings.Builder
		sb.WriteString("id,variant_id,version,build,download_count,size_bytes,sha256,created_at\n")
		for _, r := range releases {
			sb.WriteString(fmt.Sprintf("%s,%s,%s,%d,%d,%d,%s,%s\n",
				r.ID, r.VariantID, r.Version, r.Build, r.DownloadCount,
				r.SizeBytes, r.SHA256, r.CreatedAt.Format(time.RFC3339)))
		}
		c.String(http.StatusOK, sb.String())
	}
}

func handleExportEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Query("variant_id")
		from := c.Query("from")
		to := c.Query("to")

		tx := db.Model(&store.Event{}).Order("ts DESC")
		if variantID != "" {
			tx = tx.Where("variant_id = ?", variantID)
		}
		if from != "" {
			if t, err := time.Parse("2006-01-02", from); err == nil {
				tx = tx.Where("ts >= ?", t)
			}
		}
		if to != "" {
			if t, err := time.Parse("2006-01-02", to); err == nil {
				tx = tx.Where("ts < ?", t.AddDate(0, 0, 1))
			}
		}

		var events []store.Event
		tx.Find(&events)

		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=events.csv")

		var sb strings.Builder
		sb.WriteString("ts,type,variant_id,release_id,ip,ua\n")
		for _, e := range events {
			sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
				e.Ts.Format(time.RFC3339), e.Type, e.VariantID, e.ReleaseID, e.IP, csvEscape(e.UA)))
		}
		c.String(http.StatusOK, sb.String())
	}
}

func handleExportIOSDevices(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Query("variant_id")
		q := c.Query("q")

		tx := db.Model(&store.IOSDevice{}).Order("created_at DESC")
		if q != "" {
			tx = tx.Where("ud_id LIKE ? OR device_name LIKE ?", "%"+q+"%", "%"+q+"%")
		}

		if variantID != "" {
			var bindings []store.DeviceAppBinding
			db.Where("variant_id = ?", variantID).Find(&bindings)
			deviceIDs := make([]string, 0, len(bindings))
			for _, b := range bindings {
				deviceIDs = append(deviceIDs, b.DeviceID)
			}
			if len(deviceIDs) > 0 {
				tx = tx.Where("id IN ?", deviceIDs)
			} else {
				tx = tx.Where("1 = 0") // No results
			}
		}

		var devices []store.IOSDevice
		tx.Find(&devices)

		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=ios_devices.csv")

		var sb strings.Builder
		sb.WriteString("udid,device_name,model,os_version,apple_registered,created_at\n")
		for _, d := range devices {
			sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%t,%s\n",
				d.UDID, csvEscape(d.DeviceName), csvEscape(d.Model),
				d.OSVersion, d.AppleRegistered, d.CreatedAt.Format(time.RFC3339)))
		}
		c.String(http.StatusOK, sb.String())
	}
}

func queryInt(c *gin.Context, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
