package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
	"gorm.io/gorm"
)

func handleUploadConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		uploadDomain := cfg.GetPrimaryDomain()
		ok(c, gin.H{"upload_domain": uploadDomain})
	}
}

func handleUpload(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.PostForm("variant_id")
		version := c.PostForm("version")
		buildStr := c.PostForm("build")
		channel := c.PostForm("channel")
		minOS := c.PostForm("min_os")
		changelog := c.PostForm("changelog")

		if variantID == "" {
			fail(c, http.StatusBadRequest, "bad_request", "variant_id is required")
			return
		}
		if version == "" {
			fail(c, http.StatusBadRequest, "bad_request", "version is required")
			return
		}

		var variant store.Variant
		if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "variant not found")
			return
		}

		file, header, err := c.Request.FormFile("app_file")
		if err != nil {
			fail(c, http.StatusBadRequest, "bad_request", "app_file is required")
			return
		}
		defer file.Close()

		var buildNum int64
		if buildStr != "" {
			if n, parseErr := strconv.ParseInt(buildStr, 10, 64); parseErr == nil {
				buildNum = n
			}
		}

		// Save file to disk
		releaseID := newID("rel_")
		ext := filepath.Ext(header.Filename)
		storagePath := filepath.Join("uploads", variant.ProductID, variantID, releaseID, "app"+ext)
		fullPath := filepath.Join(cfg.Server.DataDir, storagePath)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			fail(c, http.StatusInternalServerError, "io_error", "failed to create upload directory")
			return
		}

		dst, err := os.Create(fullPath)
		if err != nil {
			fail(c, http.StatusInternalServerError, "io_error", "failed to create file")
			return
		}

		h := sha256.New()
		size, err := io.Copy(dst, io.TeeReader(file, h))
		dst.Close()
		if err != nil {
			fail(c, http.StatusInternalServerError, "io_error", "failed to write file")
			return
		}

		// Handle icon if provided
		iconBase64 := c.PostForm("icon_base64")
		if iconBase64 != "" {
			saveProductIcon(cfg, db, variant.ProductID, iconBase64)
		}

		// Legacy app_id: use variant's legacy app id if present, else variant id
		appID := variantID
		if variant.LegacyAppID != nil {
			appID = *variant.LegacyAppID
		}

		release := store.Release{
			ID:          releaseID,
			AppID:       appID,
			VariantID:   variantID,
			Version:     version,
			Build:       buildNum,
			Changelog:   changelog,
			MinOS:       minOS,
			SizeBytes:   size,
			SHA256:      hex.EncodeToString(h.Sum(nil)),
			StoragePath: storagePath,
			FileName:    header.Filename,
			FileExt:     strings.TrimPrefix(ext, "."),
			MimeType:    detectMimeType(ext),
			Channel:     channel,
		}
		if err := db.Create(&release).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		// Log upload event
		db.Create(&store.Event{
			Type:      "upload",
			VariantID: variantID,
			ReleaseID: releaseID,
			IP:        c.ClientIP(),
			UA:        c.GetHeader("User-Agent"),
		})

		// Build response URLs
		domain := cfg.GetPrimaryDomain()
		var product store.Product
		db.Where("id = ?", variant.ProductID).First(&product)

		ok(c, gin.H{
			"release": release,
			"urls": gin.H{
				"page":     fmt.Sprintf("%s/products/%s", domain, product.Slug),
				"download": fmt.Sprintf("%s/d/%s", domain, releaseID),
			},
		})
	}
}

func handleParseApp() gin.HandlerFunc {
	return func(c *gin.Context) {
		file, header, err := c.Request.FormFile("app_file")
		if err != nil {
			fail(c, http.StatusBadRequest, "bad_request", "app_file is required")
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))

		switch ext {
		case ".apk":
			info, parseErr := parseAPK(file, header.Size)
			if parseErr != nil {
				fail(c, http.StatusBadRequest, "parse_error", "failed to parse APK: "+parseErr.Error())
				return
			}
			ok(c, info)
		case ".ipa":
			info, parseErr := parseIPA(file, header.Size)
			if parseErr != nil {
				fail(c, http.StatusBadRequest, "parse_error", "failed to parse IPA: "+parseErr.Error())
				return
			}
			ok(c, info)
		default:
			fail(c, http.StatusBadRequest, "bad_request", "unsupported file type: "+ext)
		}
	}
}

func detectMimeType(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "ipa":
		return "application/octet-stream"
	case "apk":
		return "application/vnd.android.package-archive"
	case "dmg":
		return "application/x-apple-diskimage"
	case "exe", "msi":
		return "application/octet-stream"
	case "appimage":
		return "application/x-executable"
	case "deb":
		return "application/vnd.debian.binary-package"
	case "rpm":
		return "application/x-rpm"
	default:
		return "application/octet-stream"
	}
}

func saveProductIcon(cfg *config.Config, db *gorm.DB, productID, iconB64 string) {
	// Strip data URI prefix if present
	if idx := strings.Index(iconB64, ","); idx >= 0 {
		iconB64 = iconB64[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(iconB64)
	if err != nil {
		return
	}

	iconDir := filepath.Join(cfg.Server.DataDir, "uploads", productID)
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return
	}

	iconPath := filepath.Join(iconDir, "icon.png")
	if err := os.WriteFile(iconPath, data, 0o644); err != nil {
		return
	}

	db.Model(&store.Product{}).Where("id = ?", productID).
		Update("icon_path", "/uploads/"+productID+"/icon.png")
}
