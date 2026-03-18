package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/apple"
	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
	"gorm.io/gorm"
)

func handleListIOSVariants(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var variants []store.Variant
		db.Where("platform = ?", "ios").Order("created_at DESC").Find(&variants)

		type item struct {
			ID          string `json:"id"`
			ProductID   string `json:"product_id"`
			ProductName string `json:"product_name"`
			Identifier  string `json:"identifier"`
			DisplayName string `json:"display_name"`
			Platform    string `json:"platform"`
		}

		items := make([]item, 0, len(variants))
		for _, v := range variants {
			var product store.Product
			db.Where("id = ?", v.ProductID).First(&product)
			items = append(items, item{
				ID:          v.ID,
				ProductID:   v.ProductID,
				ProductName: product.Name,
				Identifier:  v.Identifier,
				DisplayName: v.DisplayName,
				Platform:    v.Platform,
			})
		}

		ok(c, gin.H{"items": items})
	}
}

func handleListIOSDevices(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Query("variant_id")
		q := c.Query("q")
		from := c.Query("from")
		to := c.Query("to")
		limit := queryInt(c, "limit", 50)
		offset := queryInt(c, "offset", 0)

		tx := db.Model(&store.IOSDevice{}).Order("created_at DESC")

		if q != "" {
			tx = tx.Where("ud_id LIKE ? OR device_name LIKE ?", "%"+q+"%", "%"+q+"%")
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
				tx = tx.Where("1 = 0")
			}
		}

		var total int64
		tx.Count(&total)

		var devices []store.IOSDevice
		tx.Limit(limit).Offset(offset).Find(&devices)

		ok(c, gin.H{"items": devices, "total": total})
	}
}

func handleAppleStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err != nil {
			ok(c, gin.H{
				"configured": false,
				"connected":  false,
				"message":    "settings not found",
			})
			return
		}

		configured := settings.AppleKeyID != "" && settings.AppleIssuerID != "" && settings.ApplePrivateKey != ""
		if !configured {
			ok(c, gin.H{
				"configured": false,
				"connected":  false,
				"message":    "Apple Developer API not configured",
			})
			return
		}

		// Try to connect
		client, err := apple.NewClient(settings.AppleKeyID, settings.AppleIssuerID, settings.ApplePrivateKey, settings.AppleTeamID)
		if err != nil {
			ok(c, gin.H{
				"configured": true,
				"connected":  false,
				"message":    "failed to initialize client: " + err.Error(),
			})
			return
		}

		if err := client.TestConnection(); err != nil {
			ok(c, gin.H{
				"configured": true,
				"connected":  false,
				"message":    "API connection failed: " + err.Error(),
			})
			return
		}

		ok(c, gin.H{
			"configured": true,
			"connected":  true,
			"message":    "connected",
		})
	}
}

func handleRegisterApple(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := c.Param("id")

		var device store.IOSDevice
		if err := db.Where("id = ?", deviceID).First(&device).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "device not found")
			return
		}

		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		client, err := apple.NewClient(settings.AppleKeyID, settings.AppleIssuerID, settings.ApplePrivateKey, settings.AppleTeamID)
		if err != nil {
			fail(c, http.StatusInternalServerError, "apple_error", err.Error())
			return
		}

		registeredDevice, err := client.RegisterDevice(device.UDID, device.DeviceName)
		if err != nil {
			fail(c, http.StatusInternalServerError, "apple_error", "failed to register device: "+err.Error())
			return
		}

		now := time.Now()
		db.Model(&device).Updates(map[string]interface{}{
			"apple_registered":    true,
			"apple_registered_at": &now,
			"apple_device_id":     registeredDevice.ID,
		})

		ok(c, gin.H{"apple_device_id": registeredDevice.ID})
	}
}

func handleBatchRegisterApple(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			DeviceIDs []string `json:"device_ids" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		client, err := apple.NewClient(settings.AppleKeyID, settings.AppleIssuerID, settings.ApplePrivateKey, settings.AppleTeamID)
		if err != nil {
			fail(c, http.StatusInternalServerError, "apple_error", err.Error())
			return
		}

		type result struct {
			DeviceID      string `json:"device_id"`
			Success       bool   `json:"success"`
			AppleDeviceID string `json:"apple_device_id,omitempty"`
			Error         string `json:"error,omitempty"`
		}

		results := make([]result, 0, len(req.DeviceIDs))
		successCount := 0
		failCount := 0

		for _, deviceID := range req.DeviceIDs {
			var device store.IOSDevice
			if err := db.Where("id = ?", deviceID).First(&device).Error; err != nil {
				results = append(results, result{DeviceID: deviceID, Success: false, Error: "not found"})
				failCount++
				continue
			}

			registeredDevice, regErr := client.RegisterDevice(device.UDID, device.DeviceName)
			if regErr != nil {
				results = append(results, result{DeviceID: deviceID, Success: false, Error: regErr.Error()})
				failCount++
				continue
			}

			now := time.Now()
			db.Model(&device).Updates(map[string]interface{}{
				"apple_registered":    true,
				"apple_registered_at": &now,
				"apple_device_id":     registeredDevice.ID,
			})

			results = append(results, result{DeviceID: deviceID, Success: true, AppleDeviceID: registeredDevice.ID})
			successCount++
		}

		ok(c, gin.H{
			"success_count": successCount,
			"fail_count":    failCount,
			"results":       results,
		})
	}
}

// UDID enrollment handlers

func handleUDIDEnroll(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		nonceStr := c.Param("nonce")

		var nonce store.UDIDNonce
		if err := db.Where("nonce = ? AND used = ? AND expires_at > ?", nonceStr, false, time.Now()).
			First(&nonce).Error; err != nil {
			c.String(http.StatusNotFound, "invalid or expired enrollment link")
			return
		}

		// Generate mobileconfig for UDID collection
		callbackURL := fmt.Sprintf("%s/udid/callback", cfg.GetPrimaryDomain())
		org := cfg.GetOrganization()
		bundlePrefix := cfg.GetBundleIDPrefix()

		profileUUID := generateUUID()
		payloadUUID := generateUUID()

		mobileconfig := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<dict>
		<key>URL</key>
		<string>%s?nonce=%s</string>
		<key>DeviceAttributes</key>
		<array>
			<string>UDID</string>
			<string>PRODUCT</string>
			<string>VERSION</string>
			<string>DEVICE_NAME</string>
		</array>
	</dict>
	<key>PayloadOrganization</key>
	<string>%s</string>
	<key>PayloadDisplayName</key>
	<string>%s Device Registration</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
	<key>PayloadUUID</key>
	<string>%s</string>
	<key>PayloadIdentifier</key>
	<string>%s.profile-service</string>
	<key>PayloadType</key>
	<string>Profile Service</string>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>PayloadType</key>
			<string>Configuration</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
			<key>PayloadIdentifier</key>
			<string>%s.udid</string>
			<key>PayloadUUID</key>
			<string>%s</string>
			<key>PayloadDisplayName</key>
			<string>Device Registration</string>
		</dict>
	</array>
</dict>
</plist>`, callbackURL, nonceStr, org, org, profileUUID, bundlePrefix, bundlePrefix, payloadUUID)

		c.Data(http.StatusOK, "application/x-apple-aspen-config", []byte(mobileconfig))
	}
}

func handleUDIDCallback(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		nonceStr := c.Query("nonce")
		if nonceStr == "" {
			c.String(http.StatusBadRequest, "missing nonce")
			return
		}

		var nonce store.UDIDNonce
		if err := db.Where("nonce = ? AND used = ? AND expires_at > ?", nonceStr, false, time.Now()).
			First(&nonce).Error; err != nil {
			c.String(http.StatusNotFound, "invalid or expired nonce")
			return
		}

		// Read the plist body sent by iOS
		body, err := c.GetRawData()
		if err != nil {
			c.String(http.StatusBadRequest, "cannot read body")
			return
		}

		// Parse UDID from the callback plist
		udid, deviceName, model, osVersion := parseUDIDCallback(body)
		if udid == "" {
			c.String(http.StatusBadRequest, "UDID not found in callback")
			return
		}

		// Mark nonce as used
		db.Model(&nonce).Update("used", true)

		// Create or update device
		var device store.IOSDevice
		if err := db.Where("ud_id = ?", udid).First(&device).Error; err != nil {
			now := time.Now()
			device = store.IOSDevice{
				ID:         newID("dev_"),
				UDID:       udid,
				DeviceName: deviceName,
				Model:      model,
				OSVersion:  osVersion,
				VerifiedAt: &now,
				LastIP:     c.ClientIP(),
			}
			db.Create(&device)
		} else {
			now := time.Now()
			db.Model(&device).Updates(map[string]interface{}{
				"device_name": deviceName,
				"model":       model,
				"os_version":  osVersion,
				"verified_at": &now,
				"last_ip":     c.ClientIP(),
			})
		}

		// Create binding if variant specified
		variantID := nonce.VariantID
		if variantID == "" {
			variantID = nonce.AppID // legacy fallback
		}
		if variantID != "" {
			var existing store.DeviceAppBinding
			if err := db.Where("ud_id = ? AND variant_id = ?", udid, variantID).First(&existing).Error; err != nil {
				db.Create(&store.DeviceAppBinding{
					ID:        newID("bind_"),
					DeviceID:  device.ID,
					UDID:      udid,
					AppID:     nonce.AppID,
					VariantID: variantID,
				})
			}
		}

		// Redirect to success page or back to product
		domain := cfg.GetPrimaryDomain()
		c.Redirect(http.StatusFound, domain+"/udid/success")
	}
}

func parseUDIDCallback(body []byte) (udid, deviceName, model, osVersion string) {
	content := string(body)

	// The callback plist contains device attributes
	udid = extractPlistValue(content, "UDID")
	deviceName = extractPlistValue(content, "DEVICE_NAME")
	model = extractPlistValue(content, "PRODUCT")
	osVersion = extractPlistValue(content, "VERSION")
	return
}

func extractPlistValue(content, key string) string {
	keyTag := "<key>" + key + "</key>"
	idx := strings.Index(content, keyTag)
	if idx < 0 {
		return ""
	}
	rest := content[idx+len(keyTag):]
	startTag := "<string>"
	endTag := "</string>"
	start := strings.Index(rest, startTag)
	if start < 0 {
		return ""
	}
	rest = rest[start+len(startTag):]
	end := strings.Index(rest, endTag)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func generateUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(buf[0:4]),
		hex.EncodeToString(buf[4:6]),
		hex.EncodeToString(buf[6:8]),
		hex.EncodeToString(buf[8:10]),
		hex.EncodeToString(buf[10:16]))
}
