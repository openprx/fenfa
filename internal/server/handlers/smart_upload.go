package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
)

// SmartUpload handles one-step upload: saves the original file first,
// parses a copy for metadata, then creates the release with a descriptive filename.
//
// Flow:
//  1. Save the original file to a temp location (preserved intact for download)
//  2. Copy it to a separate temp file for metadata parsing
//  3. Parse the copy to extract bundle_id, version, app_name, icon, etc.
//  4. Delete the parse copy (may be corrupted by parsing)
//  5. Rename the original with: {original_name}_{datetime}_{version}.{ext}
//  6. Call doUpload to finalize (DB, S3, response)
func SmartUpload(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := strings.TrimSpace(c.PostForm("variant_id"))
		if variantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "missing variant_id"}})
			return
		}

		target, err := loadUploadTarget(db, variantID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
			return
		}

		file, err := c.FormFile("app_file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "missing app_file"}})
			return
		}

		ext := detectFileExt(file.Filename)
		if !isAllowedVariantFileExt(target.Variant.Platform, ext) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "file extension mismatch"}})
			return
		}

		// Step 1: Save original file to temp (intact copy for download)
		origTmp, err := os.CreateTemp("", "smart-orig-*"+ext)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		origPath := origTmp.Name()

		src, err := file.Open()
		if err != nil {
			origTmp.Close()
			os.Remove(origPath)
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_FILE", "message": err.Error()}})
			return
		}
		if _, err := io.Copy(origTmp, src); err != nil {
			src.Close()
			origTmp.Close()
			os.Remove(origPath)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		src.Close()
		origTmp.Close()

		// Step 2: Copy to a separate file for parsing
		parseTmp, err := os.CreateTemp("", "smart-parse-*"+ext)
		if err != nil {
			os.Remove(origPath)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		parsePath := parseTmp.Name()
		if err := copyFile(origPath, parsePath); err != nil {
			parseTmp.Close()
			os.Remove(parsePath)
			os.Remove(origPath)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		parseTmp.Close()

		// Step 3: Parse the copy to extract metadata
		var parsed gin.H
		var parseErr error
		switch target.Variant.Platform {
		case "android":
			parsed, parseErr = parseAPKFromPath(parsePath, file.Filename, file.Size)
		case "ios":
			parsed, parseErr = parseIPAFromPath(parsePath, file.Filename, file.Size)
		}

		// Step 4: Delete parse copy (may be corrupted)
		os.Remove(parsePath)

		if parseErr != nil {
			os.Remove(origPath)
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "PARSE_ERROR", "message": parseErr.Error()}})
			return
		}

		// Build uploadParams from parsed metadata
		p := uploadParams{
			VariantID:   target.Variant.ID,
			ProductID:   target.Product.ID,
			ProductSlug: target.Product.Slug,
			ProductName: target.Product.Name,
			LegacyAppID: target.LegacyAppID,
			Platform:    target.Variant.Platform,
			Identifier:  target.Variant.Identifier,
			MinOS:       target.Variant.MinOS,
		}

		if target.Variant.Platform == "ios" {
			if bundleID := getString(parsed, "bundle_id"); bundleID != "" {
				p.Identifier = bundleID
			}
			p.Version = getString(parsed, "version")
			p.Build, _ = strconv.ParseInt(getString(parsed, "build"), 10, 64)
		} else if target.Variant.Platform == "android" {
			if packageName := getString(parsed, "package_name"); packageName != "" {
				p.Identifier = packageName
			}
			p.Version = getString(parsed, "version_name")
			if vc, ok := parsed["version_code"]; ok {
				switch v := vc.(type) {
				case int:
					p.Build = int64(v)
				case int64:
					p.Build = v
				case float64:
					p.Build = int64(v)
				}
			}
		}
		p.MinOS = coalesceFirstNonEmpty(getString(parsed, "min_os"), p.MinOS)
		if target.Variant.Platform == "android" && p.MinOS == "" {
			p.MinOS = getString(parsed, "min_sdk")
		}
		p.IconBase64 = getString(parsed, "icon_base64")

		// Backfill variant identifier if it was empty
		if target.Variant.Identifier == "" && p.Identifier != "" {
			db.Model(&store.Variant{}).Where("id = ?", target.Variant.ID).Update("identifier", p.Identifier)
		}

		// Allow form fields to override auto-parsed values
		if v := c.PostForm("version"); v != "" {
			p.Version = v
		}
		if v := c.PostForm("build"); v != "" {
			p.Build, _ = strconv.ParseInt(v, 10, 64)
		}
		if v := c.PostForm("channel"); v != "" {
			p.Channel = v
		}
		if v := c.PostForm("changelog"); v != "" {
			p.Changelog = v
		}

		// Step 5: Build descriptive filename
		// Format: {original_basename}_{YYYYMMDD_HHMMSS}_{version}.{ext}
		baseName := strings.TrimSuffix(file.Filename, ext)
		// sanitize basename: keep only safe chars
		baseName = sanitizeFilename(baseName)
		if baseName == "" {
			baseName = "app"
		}
		now := time.Now()
		version := p.Version
		if version == "" {
			version = "0"
		}
		p.Filename = fmt.Sprintf("%s_%s_%s%s", baseName, now.Format("20060102_150405"), version, ext)

		// Step 6: Pass the preserved original file to doUpload
		p.SavedFilePath = origPath

		doUpload(c, db, cfg, file, p)
	}
}

// sanitizeFilename removes unsafe characters from a filename component.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func getString(h gin.H, key string) string {
	if v, ok := h[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
