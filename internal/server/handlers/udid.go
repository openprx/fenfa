package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
)

func UDIDStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check by nonce first (stored in browser localStorage, sent as query param)
		nonce := c.Query("nonce")
		appID := c.Query("app")
		variantID := c.Query("variant")
		if nonce != "" {
			var nonceRecord store.UDIDNonce
			// Check if this nonce was used (meaning device was registered)
			if err := db.Where("nonce = ? AND used = ?", nonce, true).First(&nonceRecord).Error; err == nil {
				if variantID != "" && nonceRecord.VariantID != "" && nonceRecord.VariantID != variantID {
					c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"bound": false}})
					return
				}
				if appID != "" && nonceRecord.AppID != "" && nonceRecord.AppID != appID {
					c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"bound": false}})
					return
				}
				// Nonce was used, device is bound
				c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"bound": true}})
				return
			}
		}

		// Fallback: check cookie (for backward compatibility)
		val, err := c.Cookie("udid_bound")
		if err == nil && val == "1" && variantID == "" {
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"bound": true}})
			return
		}

		// Also check if there's any device binding for this app from recent nonces
		// This helps when user refreshes page after binding
		if variantID != "" {
			var count int64
			db.Model(&store.DeviceAppBinding{}).Where("variant_id = ?", variantID).Count(&count)
			if count > 0 {
				// Keep false to avoid incorrectly binding the wrong browser session.
			}
		} else if appID != "" {
			var count int64
			// Check if any binding exists for this app in last hour (approximate)
			db.Model(&store.DeviceAppBinding{}).Where("app_id = ?", appID).Count(&count)
			if count > 0 {
				// There are bindings, but we can't know if this specific browser is bound
				// Return not bound to be safe
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"bound": false}})
	}
}

func UDIDProfile(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce := randHex(16)

		// Get system settings from database
		var settings store.SystemSettings
		primaryDomain := cfg.GetPrimaryDomain()
		organization := cfg.GetOrganization()

		if err := db.Where("id = ?", "default").First(&settings).Error; err == nil {
			// Use database settings if available
			if settings.PrimaryDomain != "" {
				primaryDomain = settings.PrimaryDomain
			}
			if settings.Organization != "" {
				organization = settings.Organization
			}
		}

		// Optional app context
		appID := c.Query("app")
		variantID := c.Query("variant")
		var app store.App
		var hasApp bool
		var variant store.Variant
		var product store.Product
		var hasVariant bool
		if variantID != "" {
			if ctx, err := loadVariantProductContext(db, variantID); err == nil {
				variant = ctx.Variant
				product = ctx.Product
				hasVariant = true
				if variant.LegacyAppID != nil {
					appID = *variant.LegacyAppID
				}
			}
		}
		if appID != "" {
			if err := db.Where("id = ?", appID).First(&app).Error; err == nil {
				hasApp = true
				if !hasVariant {
					if ctx, err := loadVariantContextByLegacyAppID(db, app.ID); err == nil {
						variant = ctx.Variant
						product = ctx.Product
						hasVariant = true
						variantID = variant.ID
					}
				}
			}
		}

		// Save nonce to database for verification (expires in 10 minutes)
		nonceRecord := store.UDIDNonce{
			Nonce:     nonce,
			AppID:     appID,
			VariantID: variantID,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
			Used:      false,
		}
		_ = db.Create(&nonceRecord).Error

		// Set cookie with nonce so frontend can check status later
		// This cookie is set in the browser when downloading the profile
		secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
		c.SetCookie("udid_nonce", nonce, 600, "/", "", secure, false) // 10 min expiry, httpOnly=false so JS can read

		// Use primary domain for callback URL (for Apple communication); propagate app context
		cb := primaryDomain + "/udid/callback?nonce=" + nonce
		if hasVariant {
			cb += "&variant=" + variant.ID
		}
		if hasApp {
			cb += "&app=" + app.ID
		}

		// Build payload fields based on app and settings
		payloadIdentifier := ""
		displayName := organization + " - Device Registration"
		description := "Install this profile to register your device for app installation. Your device information will be collected for authorization purposes."
		deviceAttrs := []string{"UDID", "PRODUCT", "VERSION", "DEVICE_NAME"}
		if hasVariant {
			if variant.Platform == "ios" && variant.Identifier != "" {
				payloadIdentifier = variant.Identifier + ".profile-service"
			}
			name := product.Name
			if name == "" && hasApp {
				name = app.Name
			}
			if name != "" {
				displayName = organization + " - " + name
				description = "Register your device to install: " + name
			}
		} else if hasApp {
			if app.Platform == "ios" && app.BundleID != "" {
				payloadIdentifier = app.BundleID + ".profile-service"
			}
			if app.Name != "" {
				displayName = organization + " - " + app.Name
				description = "Register your device to install: " + app.Name
			}
		}

		c.Header("Content-Type", "application/x-apple-aspen-config")
		c.Header("Content-Disposition", "attachment; filename=\"profile.mobileconfig\"")
		c.String(http.StatusOK, mobileconfig(cb, organization, payloadIdentifier, displayName, description, deviceAttrs, nonce))
	}
}

// DeviceInfo holds parsed device information from UDID callback
type DeviceInfo struct {
	UDID       string
	Product    string
	Version    string
	DeviceName string
}

// extractDeviceInfo parses all device attributes from the callback plist
func extractDeviceInfo(s string) DeviceInfo {
	info := DeviceInfo{}
	// Parse UDID
	if m := regexp.MustCompile(`(?i)<key>UDID</key>\s*<string>([A-Fa-f0-9-]+)</string>`).FindStringSubmatch(s); len(m) > 1 {
		info.UDID = m[1]
	}
	// Parse PRODUCT (device model identifier like iPhone14,2)
	if m := regexp.MustCompile(`(?i)<key>PRODUCT</key>\s*<string>([^<]+)</string>`).FindStringSubmatch(s); len(m) > 1 {
		info.Product = m[1]
	}
	// Parse VERSION (iOS version)
	if m := regexp.MustCompile(`(?i)<key>VERSION</key>\s*<string>([^<]+)</string>`).FindStringSubmatch(s); len(m) > 1 {
		info.Version = m[1]
	}
	// Parse DEVICE_NAME (user's device name)
	if m := regexp.MustCompile(`(?i)<key>DEVICE_NAME</key>\s*<string>([^<]+)</string>`).FindStringSubmatch(s); len(m) > 1 {
		info.DeviceName = m[1]
	}
	return info
}

func UDIDCallback(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log incoming callback request
		clientIP := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		log.Printf("[UDID Callback] Received request from IP=%s UA=%s", clientIP, userAgent)

		// Verify nonce and get app context
		nonce := c.Query("nonce")
		appID := c.Query("app")
		variantID := c.Query("variant")
		log.Printf("[UDID Callback] Query params: nonce=%s app=%s variant=%s", nonce, appID, variantID)

		if nonce != "" {
			var nonceRecord store.UDIDNonce
			// First try to find unused nonce
			if err := db.Where("nonce = ? AND used = ? AND expires_at > ?", nonce, false, time.Now()).First(&nonceRecord).Error; err != nil {
				// Nonce not found as unused, check if it's already used (idempotent retry)
				var usedNonce store.UDIDNonce
				if err2 := db.Where("nonce = ? AND used = ?", nonce, true).First(&usedNonce).Error; err2 == nil {
					// Nonce was already used - this is a retry from iOS
					// Return success (301 redirect) to satisfy iOS Profile Service
					log.Printf("[UDID Callback] Nonce already used (retry): %s, returning 301", nonce)
					appID = usedNonce.AppID
					variantID = usedNonce.VariantID
					primaryDomain := cfg.GetPrimaryDomain()
					var settings store.SystemSettings
					if err := db.Where("id = ?", "default").First(&settings).Error; err == nil {
						if settings.PrimaryDomain != "" {
							primaryDomain = settings.PrimaryDomain
						}
					}
					redirectURL := buildUDIDRedirectURL(db, primaryDomain, variantID, appID)
					respondPermanentRedirect(c, redirectURL)
					return
				}
				log.Printf("[UDID Callback] ERROR: Invalid or expired nonce=%s error=%v", nonce, err)
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "INVALID_NONCE", "message": "invalid or expired nonce"}})
				return
			}
			// Mark nonce as used
			db.Model(&nonceRecord).Update("used", true)
			log.Printf("[UDID Callback] Nonce verified and marked as used: %s", nonce)
			// Use app ID from nonce record if not in query
			if appID == "" {
				appID = nonceRecord.AppID
				log.Printf("[UDID Callback] Using appID from nonce record: %s", appID)
			}
			if variantID == "" {
				variantID = nonceRecord.VariantID
				log.Printf("[UDID Callback] Using variantID from nonce record: %s", variantID)
			}
		}

		b, _ := io.ReadAll(c.Request.Body)
		bodyLen := len(b)
		log.Printf("[UDID Callback] Received body length=%d bytes", bodyLen)
		// Debug: log raw body to see what iOS sends
		log.Printf("[UDID Callback] Raw body: %s", string(b))

		info := extractDeviceInfo(string(b))
		if info.UDID == "" {
			log.Printf("[UDID Callback] ERROR: UDID not found in request body. Body preview: %s", truncateString(string(b), 500))
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "udid not found"}})
			return
		}

		log.Printf("[UDID Callback] Parsed device info: UDID=%s Product=%s Version=%s DeviceName=%s",
			info.UDID, info.Product, info.Version, info.DeviceName)

		var dev store.IOSDevice
		err := db.Where("ud_id = ?", info.UDID).First(&dev).Error
		now := time.Now()
		if err != nil {
			// Create new device with full info
			dev = store.IOSDevice{
				ID:         "dev_" + randHex(8),
				UDID:       info.UDID,
				Model:      info.Product,
				OSVersion:  info.Version,
				DeviceName: info.DeviceName,
				CreatedAt:  now,
				LastIP:     clientIP,
			}
			if createErr := db.Create(&dev).Error; createErr != nil {
				log.Printf("[UDID Callback] ERROR: Failed to create device: %v", createErr)
			} else {
				log.Printf("[UDID Callback] Created new device: ID=%s UDID=%s", dev.ID, info.UDID)
			}
		} else {
			// Update existing device with latest info
			updates := map[string]any{
				"last_ip":     clientIP,
				"verified_at": now,
			}
			if info.Product != "" {
				updates["model"] = info.Product
			}
			if info.Version != "" {
				updates["os_version"] = info.Version
			}
			if info.DeviceName != "" {
				updates["device_name"] = info.DeviceName
			}
			if updateErr := db.Model(&dev).Updates(updates).Error; updateErr != nil {
				log.Printf("[UDID Callback] ERROR: Failed to update device: %v", updateErr)
			} else {
				log.Printf("[UDID Callback] Updated existing device: ID=%s UDID=%s", dev.ID, info.UDID)
			}
		}

		// Create device-app binding if app context exists
		if variantID != "" || appID != "" {
			if variantID == "" && appID != "" {
				if ctx, err := loadVariantContextByLegacyAppID(db, appID); err == nil {
					variantID = ctx.Variant.ID
				}
			}
			var existing store.DeviceAppBinding
			query := db.Where("ud_id = ?", info.UDID)
			if variantID != "" {
				query = query.Where("variant_id = ?", variantID)
			} else {
				query = query.Where("app_id = ?", appID)
			}
			if err := query.First(&existing).Error; err != nil {
				// Create new binding
				binding := store.DeviceAppBinding{
					ID:        "bind_" + randHex(8),
					DeviceID:  dev.ID,
					UDID:      info.UDID,
					AppID:     appID,
					VariantID: variantID,
					CreatedAt: now,
				}
				if bindErr := db.Create(&binding).Error; bindErr != nil {
					log.Printf("[UDID Callback] ERROR: Failed to create binding: %v", bindErr)
				} else {
					log.Printf("[UDID Callback] Created device-app binding: DeviceID=%s AppID=%s VariantID=%s", dev.ID, appID, variantID)
				}
			} else {
				log.Printf("[UDID Callback] Device-app binding already exists: DeviceID=%s AppID=%s VariantID=%s", dev.ID, appID, variantID)
			}
		}

		// Set cookie with proper secure flag based on connection
		secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
		c.SetCookie("udid_bound", "1", 86400*365, "/", "", secure, true)
		log.Printf("[UDID Callback] Set udid_bound cookie, secure=%v", secure)

		// iOS Profile Service requires 301 redirect (NOT 302 or 200 OK)
		// 301 will automatically open Safari to the redirect URL
		// Using 302 or 200 OK causes "invalid profile" error
		// Reference: https://github.com/shaojiankui/iOS-UDID-Safari
		primaryDomain := cfg.GetPrimaryDomain()
		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err == nil {
			if settings.PrimaryDomain != "" {
				primaryDomain = settings.PrimaryDomain
			}
		}

		redirectURL := buildUDIDRedirectURL(db, primaryDomain, variantID, appID)
		log.Printf("[UDID Callback] SUCCESS: UDID=%s registered, 301 redirect to %s", info.UDID, redirectURL)
		respondPermanentRedirect(c, redirectURL)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractUDID(s string) string {
	re := regexp.MustCompile(`(?i)<key>UDID</key>\s*<string>([A-F0-9-]+)</string>`) // simple plist parse
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateUUID generates a standard UUID v4 (RFC 4122) in UPPERCASE
// Format: XXXXXXXX-XXXX-4XXX-YXXX-XXXXXXXXXXXX
// where X is any hexadecimal digit and Y is one of 8, 9, A, or B
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	// Set version (4) and variant bits according to RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10

	// Use uppercase hex encoding (Apple standard)
	uuid := hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16])

	// Convert to uppercase
	return toUpperHex(uuid)
}

// toUpperHex converts hex string to uppercase
func toUpperHex(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'f' {
			result[i] = c - 32 // Convert to uppercase
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func xmlEscape(s string) string {
	// Escape special characters for XML text nodes
	return html.EscapeString(s)
}

func loadVariantContextByLegacyAppID(db *gorm.DB, appID string) (releaseVariantContext, error) {
	var variant store.Variant
	if err := db.Where("legacy_app_id = ?", appID).First(&variant).Error; err != nil {
		return releaseVariantContext{}, err
	}
	var product store.Product
	if err := db.Where("id = ?", variant.ProductID).First(&product).Error; err != nil {
		return releaseVariantContext{}, err
	}
	return releaseVariantContext{Variant: variant, Product: product}, nil
}

func buildUDIDRedirectURL(db *gorm.DB, primaryDomain, variantID, appID string) string {
	if variantID != "" {
		if ctx, err := loadVariantProductContext(db, variantID); err == nil {
			return primaryDomain + "/products/" + productPathKey(ctx.Product) + "?udid_bound=1&variant=" + url.QueryEscape(variantID)
		}
	}
	if appID != "" {
		return primaryDomain + "/apps/" + appID + "?udid_bound=1"
	}
	return primaryDomain + "/?udid_bound=1"
}

func respondPermanentRedirect(c *gin.Context, redirectURL string) {
	c.Header("Location", redirectURL)
	c.Status(http.StatusMovedPermanently)
	c.Writer.WriteHeaderNow()
}

func mobileconfig(callbackURL, organization, payloadIdentifier, displayName, description string, deviceAttrs []string, challenge string) string {
	// Generate proper UUID (Apple requires UPPERCASE)
	payloadUUID := generateUUID()

	// Set defaults
	if organization == "" {
		organization = "Device Registration"
	}
	if displayName == "" {
		displayName = "Profile Service"
	}
	if description == "" {
		description = "Install this profile to register your device for app installation."
	}
	if payloadIdentifier == "" {
		payloadIdentifier = "com.example.profile-service"
	}
	if len(deviceAttrs) == 0 {
		deviceAttrs = []string{"UDID", "PRODUCT", "VERSION", "DEVICE_NAME", "SERIAL"}
	}

	// Escape values for XML text nodes
	escCallback := xmlEscape(callbackURL)
	escOrg := xmlEscape(organization)
	escDisplay := xmlEscape(displayName)
	escDescription := xmlEscape(description)
	escPayloadUUID := xmlEscape(payloadUUID)
	escPayloadIdentifier := xmlEscape(payloadIdentifier)

	// Build DeviceAttributes XML
	attrsXML := ""
	for _, a := range deviceAttrs {
		attrsXML += "\n\t\t<string>" + xmlEscape(a) + "</string>"
	}

	// Optional Challenge
	challengeXML := ""
	if challenge != "" {
		challengeXML = "\n\t<key>Challenge</key>\n\t<string>" + xmlEscape(challenge) + "</string>"
	}

	// Profile Service format (flat structure, NOT nested in PayloadContent array)
	// This is the correct format for UDID enrollment as per Apple documentation
	// Reference: https://github.com/MerchV/Get-iOS-UDID
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<dict>
		<key>URL</key>
		<string>` + escCallback + `</string>` + challengeXML + `
		<key>DeviceAttributes</key>
		<array>` + attrsXML + `
		</array>
	</dict>
	<key>PayloadOrganization</key>
	<string>` + escOrg + `</string>
	<key>PayloadDisplayName</key>
	<string>` + escDisplay + `</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
	<key>PayloadUUID</key>
	<string>` + escPayloadUUID + `</string>
	<key>PayloadIdentifier</key>
	<string>` + escPayloadIdentifier + `</string>
	<key>PayloadDescription</key>
	<string>` + escDescription + `</string>
	<key>PayloadType</key>
	<string>Profile Service</string>
</dict>
</plist>`
}
