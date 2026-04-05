package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

// AdminListIOSDevices returns a JSON list of iOS devices (UDIDs) with basic filters
func AdminListIOSDevices(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Filter by app_id / variant_id (uses DeviceAppBinding table)
		appID := c.Query("app_id")
		variantID := c.Query("variant_id")
		var q *gorm.DB

		if variantID != "" {
			q = db.Model(&store.IOSDevice{}).
				Joins("INNER JOIN device_app_bindings ON device_app_bindings.ud_id = ios_devices.ud_id").
				Where("device_app_bindings.variant_id = ?", variantID)
		} else if appID != "" {
			q = db.Model(&store.IOSDevice{}).
				Joins("INNER JOIN device_app_bindings ON device_app_bindings.ud_id = ios_devices.ud_id").
				Where("device_app_bindings.app_id = ?", appID)
		} else {
			q = db.Model(&store.IOSDevice{})
		}

		// Filters: from/to by created_at
		const layout = "2006-01-02"
		if from := c.Query("from"); from != "" {
			if t, err := time.Parse(layout, from); err == nil {
				q = q.Where("ios_devices.created_at >= ?", t)
			}
		}
		if to := c.Query("to"); to != "" {
			if t, err := time.Parse(layout, to); err == nil {
				q = q.Where("ios_devices.created_at < ?", t.Add(24*time.Hour))
			}
		}

		// Text search in several columns
		if s := c.Query("q"); s != "" {
			like := "%" + s + "%"
			// Column name is ud_id in DB
			q = q.Where("ios_devices.ud_id LIKE ? OR ios_devices.device_name LIKE ? OR ios_devices.model LIKE ? OR ios_devices.os_version LIKE ?", like, like, like, like)
		}

		var total int64
		q.Count(&total)

		// Pagination
		limit := 100
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		offset := 0
		if v := c.Query("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}

		var ds []store.IOSDevice
		if err := q.Order("ios_devices.created_at desc").Limit(limit).Offset(offset).Find(&ds).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": []gin.H{}, "total": 0}})
			return
		}

		items := make([]gin.H, 0, len(ds))
		for _, d := range ds {
			items = append(items, gin.H{
				"id":                   d.ID,
				"udid":                 d.UDID,
				"device_name":          d.DeviceName,
				"model":                d.Model,
				"os_version":           d.OSVersion,
				"created_at":           d.CreatedAt,
				"verified_at":          d.VerifiedAt,
				"apple_registered":     d.AppleRegistered,
				"apple_registered_at":  d.AppleRegisteredAt,
				"apple_device_id":      d.AppleDeviceID,
			})
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": items, "total": total}})
	}
}

// AdminListIOSVariants returns a list of iOS variants for the device filter dropdown
func AdminListIOSVariants(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var variants []store.Variant
		if err := db.Where("platform = ?", "ios").Order("created_at asc").Find(&variants).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": []gin.H{}}})
			return
		}

		items := make([]gin.H, 0, len(variants))
		for _, variant := range variants {
			var product store.Product
			if err := db.Where("id = ?", variant.ProductID).First(&product).Error; err != nil {
				continue
			}
			items = append(items, gin.H{
				"id":           variant.ID,
				"identifier":   variant.Identifier,
				"display_name": variant.DisplayName,
				"product_id":   product.ID,
				"product_name": product.Name,
			})
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": items}})
	}
}
