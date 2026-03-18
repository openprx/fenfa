package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/store"
	"gorm.io/gorm"
)

func handleGetSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		ok(c, gin.H{
			"primary_domain":    settings.PrimaryDomain,
			"secondary_domains": parseJSONArray(settings.SecondaryDomains),
			"organization":      settings.Organization,
			"updated_at":        settings.UpdatedAt,
			"storage_type":      settings.StorageType,
			"upload_domain":     settings.UploadDomain,
			"s3_endpoint":       settings.S3Endpoint,
			"s3_bucket":         settings.S3Bucket,
			"s3_public_url":     settings.S3PublicURL,
			"s3_configured":     settings.S3Endpoint != "" && settings.S3Bucket != "",
			"apple_configured":  settings.AppleKeyID != "" && settings.AppleIssuerID != "",
			"apple_key_id":      settings.AppleKeyID,
			"apple_team_id":     settings.AppleTeamID,
		})
	}
}

func handleUpdateSettings(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PrimaryDomain    string   `json:"primary_domain"`
			SecondaryDomains []string `json:"secondary_domains"`
			Organization     string   `json:"organization"`
			StorageType      string   `json:"storage_type"`
			UploadDomain     string   `json:"upload_domain"`
			S3Endpoint       string   `json:"s3_endpoint"`
			S3Bucket         string   `json:"s3_bucket"`
			S3AccessKey      string   `json:"s3_access_key"`
			S3SecretKey      string   `json:"s3_secret_key"`
			S3PublicURL      string   `json:"s3_public_url"`
			AppleKeyID       string   `json:"apple_key_id"`
			AppleIssuerID    string   `json:"apple_issuer_id"`
			AppleTeamID      string   `json:"apple_team_id"`
			ApplePrivateKey  string   `json:"apple_private_key"`
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

		updates := map[string]interface{}{
			"primary_domain":    req.PrimaryDomain,
			"secondary_domains": toJSONArray(req.SecondaryDomains),
			"organization":      req.Organization,
		}

		if req.StorageType != "" {
			updates["storage_type"] = req.StorageType
		}
		if req.UploadDomain != "" {
			updates["upload_domain"] = req.UploadDomain
		}
		if req.S3Endpoint != "" {
			updates["s3_endpoint"] = req.S3Endpoint
		}
		if req.S3Bucket != "" {
			updates["s3_bucket"] = req.S3Bucket
		}
		if req.S3AccessKey != "" {
			updates["s3_access_key"] = req.S3AccessKey
		}
		if req.S3SecretKey != "" {
			updates["s3_secret_key"] = req.S3SecretKey
		}
		if req.S3PublicURL != "" {
			updates["s3_public_url"] = req.S3PublicURL
		}
		if req.AppleKeyID != "" {
			updates["apple_key_id"] = req.AppleKeyID
		}
		if req.AppleIssuerID != "" {
			updates["apple_issuer_id"] = req.AppleIssuerID
		}
		if req.AppleTeamID != "" {
			updates["apple_team_id"] = req.AppleTeamID
		}
		if req.ApplePrivateKey != "" {
			updates["apple_private_key"] = req.ApplePrivateKey
		}

		if err := db.Model(&settings).Updates(updates).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		// Return updated settings
		handleGetSettings(db)(c)
	}
}

func parseJSONArray(s string) []string {
	if s == "" || s == "[]" {
		return []string{}
	}
	// Simple parsing: remove brackets, split by comma, trim quotes
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"`)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func toJSONArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	parts := make([]string, len(arr))
	for i, s := range arr {
		parts[i] = `"` + s + `"`
	}
	return "[" + strings.Join(parts, ",") + "]"
}
