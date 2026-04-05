package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

// trimDomainURL removes trailing slashes from a domain URL.
func trimDomainURL(u string) string {
	return strings.TrimRight(strings.TrimSpace(u), "/")
}

// SystemSettingsResponse represents the API response for system settings
type SystemSettingsResponse struct {
	PrimaryDomain    string   `json:"primary_domain"`
	SecondaryDomains []string `json:"secondary_domains"`
	Organization     string   `json:"organization"`
	UpdatedAt        string   `json:"updated_at"`

	// Storage configuration
	StorageType  string `json:"storage_type"`
	UploadDomain string `json:"upload_domain,omitempty"`

	// S3/R2 configuration (no secrets)
	S3Configured bool   `json:"s3_configured"`
	S3Endpoint   string `json:"s3_endpoint,omitempty"`
	S3Bucket     string `json:"s3_bucket,omitempty"`
	S3PublicURL  string `json:"s3_public_url,omitempty"`

	// Apple API configuration status (not credentials)
	AppleConfigured bool   `json:"apple_configured"`
	AppleKeyID      string `json:"apple_key_id,omitempty"`  // Only show ID, not full credentials
	AppleTeamID     string `json:"apple_team_id,omitempty"` // Only show ID
}

// SystemSettingsUpdateRequest represents the API request for updating system settings
type SystemSettingsUpdateRequest struct {
	PrimaryDomain    string   `json:"primary_domain"`
	SecondaryDomains []string `json:"secondary_domains"`
	Organization     string   `json:"organization"`

	// Storage configuration (optional)
	StorageType  *string `json:"storage_type,omitempty"`
	UploadDomain *string `json:"upload_domain,omitempty"`

	// S3/R2 configuration (optional)
	S3Endpoint  *string `json:"s3_endpoint,omitempty"`
	S3Bucket    *string `json:"s3_bucket,omitempty"`
	S3AccessKey *string `json:"s3_access_key,omitempty"`
	S3SecretKey *string `json:"s3_secret_key,omitempty"`
	S3PublicURL *string `json:"s3_public_url,omitempty"`

	// Apple API credentials (optional)
	AppleKeyID      string `json:"apple_key_id,omitempty"`
	AppleIssuerID   string `json:"apple_issuer_id,omitempty"`
	ApplePrivateKey string `json:"apple_private_key,omitempty"` // PEM format
	AppleTeamID     string `json:"apple_team_id,omitempty"`
}

// GetSystemSettings returns the current system settings
func GetSystemSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "SETTINGS_NOT_FOUND",
					"message": "System settings not found",
				},
			})
			return
		}

		// Parse secondary domains from JSON string
		var secondaryDomains []string
		if settings.SecondaryDomains != "" {
			_ = json.Unmarshal([]byte(settings.SecondaryDomains), &secondaryDomains)
		}
		if secondaryDomains == nil {
			secondaryDomains = []string{}
		}

		// Check if Apple API is configured
		appleConfigured := settings.AppleKeyID != "" && settings.AppleIssuerID != "" && settings.ApplePrivateKey != ""

		// Check if S3 is configured
		s3Configured := settings.S3Endpoint != "" && settings.S3Bucket != "" && settings.S3AccessKey != "" && settings.S3SecretKey != ""

		storageType := settings.StorageType
		if storageType == "" {
			storageType = "local"
		}

		response := SystemSettingsResponse{
			PrimaryDomain:    settings.PrimaryDomain,
			SecondaryDomains: secondaryDomains,
			Organization:     settings.Organization,
			UpdatedAt:        settings.UpdatedAt.Format(time.RFC3339),
			StorageType:      storageType,
			UploadDomain:     settings.UploadDomain,
			S3Configured:     s3Configured,
			S3Endpoint:       settings.S3Endpoint,
			S3Bucket:         settings.S3Bucket,
			S3PublicURL:      settings.S3PublicURL,
			AppleConfigured:  appleConfigured,
			AppleKeyID:       settings.AppleKeyID,
			AppleTeamID:      settings.AppleTeamID,
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": response,
		})
	}
}

// UpdateSystemSettings updates the system settings
func UpdateSystemSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SystemSettingsUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "Invalid request body",
				},
			})
			return
		}

		// Normalize domain URLs: trim trailing slashes
		req.PrimaryDomain = trimDomainURL(req.PrimaryDomain)
		for i, d := range req.SecondaryDomains {
			req.SecondaryDomains[i] = trimDomainURL(d)
		}
		if req.UploadDomain != nil {
			v := trimDomainURL(*req.UploadDomain)
			req.UploadDomain = &v
		}
		if req.S3PublicURL != nil {
			v := trimDomainURL(*req.S3PublicURL)
			req.S3PublicURL = &v
		}

		// Validate required fields
		if req.PrimaryDomain == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "Primary domain is required",
				},
			})
			return
		}

		if req.Organization == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "Organization is required",
				},
			})
			return
		}

		// Convert secondary domains to JSON string
		secondaryDomainsJSON := "[]"
		if req.SecondaryDomains != nil && len(req.SecondaryDomains) > 0 {
			if b, err := json.Marshal(req.SecondaryDomains); err == nil {
				secondaryDomainsJSON = string(b)
			}
		}

		// Update or create settings
		var settings store.SystemSettings
		err := db.Where("id = ?", "default").First(&settings).Error
		if err == gorm.ErrRecordNotFound {
			// Create new settings
			settings = store.SystemSettings{
				ID:               "default",
				PrimaryDomain:    req.PrimaryDomain,
				SecondaryDomains: secondaryDomainsJSON,
				Organization:     req.Organization,
				UpdatedAt:        time.Now(),
				AppleKeyID:       req.AppleKeyID,
				AppleIssuerID:    req.AppleIssuerID,
				ApplePrivateKey:  req.ApplePrivateKey,
				AppleTeamID:      req.AppleTeamID,
			}
			if req.StorageType != nil {
				settings.StorageType = *req.StorageType
			}
			if req.UploadDomain != nil {
				settings.UploadDomain = *req.UploadDomain
			}
			if req.S3Endpoint != nil {
				settings.S3Endpoint = *req.S3Endpoint
			}
			if req.S3Bucket != nil {
				settings.S3Bucket = *req.S3Bucket
			}
			if req.S3AccessKey != nil {
				settings.S3AccessKey = *req.S3AccessKey
			}
			if req.S3SecretKey != nil {
				settings.S3SecretKey = *req.S3SecretKey
			}
			if req.S3PublicURL != nil {
				settings.S3PublicURL = *req.S3PublicURL
			}
			if err := db.Create(&settings).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"ok": false,
					"error": gin.H{
						"code":    "DATABASE_ERROR",
						"message": "Failed to create settings",
					},
				})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "DATABASE_ERROR",
					"message": "Failed to query settings",
				},
			})
			return
		} else {
			// Update existing settings
			updates := map[string]interface{}{
				"primary_domain":    req.PrimaryDomain,
				"secondary_domains": secondaryDomainsJSON,
				"organization":      req.Organization,
				"updated_at":        time.Now(),
			}
			// Only update storage fields if provided
			if req.StorageType != nil {
				updates["storage_type"] = *req.StorageType
			}
			if req.UploadDomain != nil {
				updates["upload_domain"] = *req.UploadDomain
			}
			if req.S3Endpoint != nil {
				updates["s3_endpoint"] = *req.S3Endpoint
			}
			if req.S3Bucket != nil {
				updates["s3_bucket"] = *req.S3Bucket
			}
			if req.S3AccessKey != nil {
				updates["s3_access_key"] = *req.S3AccessKey
			}
			if req.S3SecretKey != nil {
				updates["s3_secret_key"] = *req.S3SecretKey
			}
			if req.S3PublicURL != nil {
				updates["s3_public_url"] = *req.S3PublicURL
			}
			// Only update Apple credentials if provided (allow partial updates)
			if req.AppleKeyID != "" {
				updates["apple_key_id"] = req.AppleKeyID
			}
			if req.AppleIssuerID != "" {
				updates["apple_issuer_id"] = req.AppleIssuerID
			}
			if req.ApplePrivateKey != "" {
				updates["apple_private_key"] = req.ApplePrivateKey
			}
			if req.AppleTeamID != "" {
				updates["apple_team_id"] = req.AppleTeamID
			}
			if err := db.Model(&settings).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"ok": false,
					"error": gin.H{
						"code":    "DATABASE_ERROR",
						"message": "Failed to update settings",
					},
				})
				return
			}
		}

		// Re-read settings to get updated values
		db.Where("id = ?", "default").First(&settings)

		// Return updated settings
		var updatedSecondaryDomains []string
		if settings.SecondaryDomains != "" {
			_ = json.Unmarshal([]byte(settings.SecondaryDomains), &updatedSecondaryDomains)
		}
		if updatedSecondaryDomains == nil {
			updatedSecondaryDomains = []string{}
		}

		// Check if Apple API is configured
		appleConfigured := settings.AppleKeyID != "" && settings.AppleIssuerID != "" && settings.ApplePrivateKey != ""

		// Check if S3 is configured
		s3Configured := settings.S3Endpoint != "" && settings.S3Bucket != "" && settings.S3AccessKey != "" && settings.S3SecretKey != ""

		storageType := settings.StorageType
		if storageType == "" {
			storageType = "local"
		}

		response := SystemSettingsResponse{
			PrimaryDomain:    settings.PrimaryDomain,
			SecondaryDomains: updatedSecondaryDomains,
			Organization:     settings.Organization,
			UpdatedAt:        settings.UpdatedAt.Format(time.RFC3339),
			StorageType:      storageType,
			UploadDomain:     settings.UploadDomain,
			S3Configured:     s3Configured,
			S3Endpoint:       settings.S3Endpoint,
			S3Bucket:         settings.S3Bucket,
			S3PublicURL:      settings.S3PublicURL,
			AppleConfigured:  appleConfigured,
			AppleKeyID:       settings.AppleKeyID,
			AppleTeamID:      settings.AppleTeamID,
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": response,
		})
	}
}

// GetUploadConfig returns minimal upload configuration for the frontend
func GetUploadConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings store.SystemSettings
		db.Where("id = ?", "default").First(&settings)

		storageType := settings.StorageType
		if storageType == "" {
			storageType = "local"
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"data": gin.H{
				"storage_type":  storageType,
				"upload_domain": settings.UploadDomain,
			},
		})
	}
}
