package handlers

import (
	"encoding/csv"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

func ExportReleases(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=\"releases.csv\"")
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"app_id", "variant_id", "release_id", "version", "build", "created_at", "size_bytes", "sha256", "download_count", "channel"})

		q := db.Model(&store.Release{})
		if appID := c.Query("app_id"); appID != "" { q = q.Where("app_id = ?", appID) }
		if variantID := c.Query("variant_id"); variantID != "" { q = q.Where("variant_id = ?", variantID) }
		if from, to := c.Query("from"), c.Query("to"); from != "" || to != "" {
			const layout = "2006-01-02"
			if from != "" { if t, err := time.Parse(layout, from); err == nil { q = q.Where("created_at >= ?", t) } }
			if to != "" { if t, err := time.Parse(layout, to); err == nil { q = q.Where("created_at < ?", t.Add(24*time.Hour)) } }
		}
		var rs []store.Release
		if err := q.Order("created_at desc").Find(&rs).Error; err == nil {
			for _, r := range rs {
				_ = w.Write([]string{r.AppID, r.VariantID, r.ID, r.Version, itoa(r.Build), r.CreatedAt.Format("2006-01-02 15:04:05"), itoa64(r.SizeBytes), r.SHA256, itoa64(r.DownloadCount), r.Channel})
			}
		}
		w.Flush()
	}
}

func ExportEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=\"events.csv\"")
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"ts", "type", "app_id", "variant_id", "release_id", "ip", "ua", "extra"})

		q := db.Model(&store.Event{})
		if appID := c.Query("app_id"); appID != "" { q = q.Where("app_id = ?", appID) }
		if variantID := c.Query("variant_id"); variantID != "" { q = q.Where("variant_id = ?", variantID) }
		if releaseID := c.Query("release_id"); releaseID != "" { q = q.Where("release_id = ?", releaseID) }
		if typ := c.Query("type"); typ != "" { q = q.Where("type = ?", typ) }
		if from, to := c.Query("from"), c.Query("to"); from != "" || to != "" {
			const layout = "2006-01-02"
			if from != "" { if t, err := time.Parse(layout, from); err == nil { q = q.Where("ts >= ?", t) } }
			if to != "" { if t, err := time.Parse(layout, to); err == nil { q = q.Where("ts < ?", t.Add(24*time.Hour)) } }
		}
		var evs []store.Event
		if err := q.Order("ts desc").Find(&evs).Error; err == nil {
			for _, e := range evs {
				_ = w.Write([]string{e.Ts.Format("2006-01-02 15:04:05"), e.Type, e.AppID, e.VariantID, e.ReleaseID, e.IP, e.UA, e.Extra})
			}
		}
		w.Flush()
	}
}

func ExportIOSDevices(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=\"ios_devices.csv\"")
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"udid", "device_name", "model", "os_version", "created_at", "verified_at"})

		q := db.Model(&store.IOSDevice{})
		if appID := c.Query("app_id"); appID != "" {
			q = q.Joins("INNER JOIN device_app_bindings ON device_app_bindings.ud_id = ios_devices.ud_id").
				Where("device_app_bindings.app_id = ?", appID)
		}
		if variantID := c.Query("variant_id"); variantID != "" {
			q = q.Joins("INNER JOIN device_app_bindings ON device_app_bindings.ud_id = ios_devices.ud_id").
				Where("device_app_bindings.variant_id = ?", variantID)
		}
		if from, to := c.Query("from"), c.Query("to"); from != "" || to != "" {
			const layout = "2006-01-02"
			if from != "" { if t, err := time.Parse(layout, from); err == nil { q = q.Where("created_at >= ?", t) } }
			if to != "" { if t, err := time.Parse(layout, to); err == nil { q = q.Where("created_at < ?", t.Add(24*time.Hour)) } }
		}
		var ds []store.IOSDevice
		if err := q.Order("created_at desc").Find(&ds).Error; err == nil {
			for _, d := range ds {
				ver := ""
				if d.VerifiedAt != nil { ver = d.VerifiedAt.Format("2006-01-02 15:04:05") }
				_ = w.Write([]string{d.UDID, d.DeviceName, d.Model, d.OSVersion, d.CreatedAt.Format("2006-01-02 15:04:05"), ver})
			}
		}
		w.Flush()
	}
}

func itoa(n int64) string { return fmt.Sprintf("%d", n) }
func itoa64(n int64) string { return fmt.Sprintf("%d", n) }
