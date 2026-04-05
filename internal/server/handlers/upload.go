package handlers

import (
	"archive/zip"
	bytes "bytes"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"howett.net/plist"

	"github.com/openprx/fenfa/internal/config"
	s3client "github.com/openprx/fenfa/internal/s3"
	"github.com/openprx/fenfa/internal/store"
)

// uploadParams holds the metadata for an upload operation.
type uploadParams struct {
	VariantID     string
	ProductID     string
	ProductSlug   string
	ProductName   string
	LegacyAppID   string
	Platform      string
	Identifier    string
	Version       string
	Build         int64
	Channel       string
	MinOS         string
	Changelog     string
	IconBase64    string
	Filename      string // custom filename; defaults to "app.{ext}" if empty
	SavedFilePath string // pre-saved file path; skip multipart read if set
}

// Upload handles multipart app package upload.
func Upload(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := strings.TrimSpace(c.PostForm("variant_id"))
		if variantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "missing variant_id"}})
			return
		}
		file, err := c.FormFile("app_file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "missing app_file"}})
			return
		}

		target, err := loadUploadTarget(db, variantID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
			return
		}

		ext := detectFileExt(file.Filename)
		if !isAllowedVariantFileExt(target.Variant.Platform, ext) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "file extension mismatch"}})
			return
		}
		buildStr := c.PostForm("build")
		build, _ := strconv.ParseInt(buildStr, 10, 64)

		p := uploadParams{
			VariantID:   target.Variant.ID,
			ProductID:   target.Product.ID,
			ProductSlug: target.Product.Slug,
			ProductName: target.Product.Name,
			LegacyAppID: target.LegacyAppID,
			Platform:    target.Variant.Platform,
			Identifier:  target.Variant.Identifier,
			Version:     c.PostForm("version"),
			Build:       build,
			Channel:     c.PostForm("channel"),
			MinOS:       coalesceFirstNonEmpty(c.PostForm("min_os"), target.Variant.MinOS),
			Changelog:   c.PostForm("changelog"),
			IconBase64:  c.PostForm("icon_base64"),
		}
		doUpload(c, db, cfg, file, p)
	}
}

// doUpload is the shared upload logic used by both Upload and SmartUpload.
func doUpload(c *gin.Context, db *gorm.DB, cfg *config.Config, file *multipart.FileHeader, p uploadParams) {
	platform := p.Platform
	version := p.Version
	build := p.Build
	channel := p.Channel
	minOS := p.MinOS
	changelog := p.Changelog
	ext := detectFileExt(file.Filename)

	release := store.Release{
		ID:        "rel_" + randHexN(8),
		AppID:     p.LegacyAppID,
		VariantID: p.VariantID,
		Version:   version,
		Build:     build,
		Channel:   channel,
		MinOS:     minOS,
		Changelog: changelog,
		FileExt:   ext,
		FileName:  filepath.Base(file.Filename),
		MimeType:  contentTypeForExt(ext),
		CreatedAt: time.Now(),
	}

	// Save file and compute SHA256
	appDir := filepath.Join("uploads", p.ProductID, p.VariantID, release.ID)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
		return
	}
	filename := "app" + ext
	if p.Filename != "" {
		filename = p.Filename
	}
	destPath := filepath.Join(appDir, filename)

	if p.SavedFilePath != "" {
		// File already saved (SmartUpload flow): move to final location
		if err := os.Rename(p.SavedFilePath, destPath); err != nil {
			// cross-device fallback: copy + remove
			if cpErr := copyFile(p.SavedFilePath, destPath); cpErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": cpErr.Error()}})
				return
			}
			_ = os.Remove(p.SavedFilePath)
		}
		// Compute SHA256 from saved file
		f, err := os.Open(destPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		f.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		release.SHA256 = hex.EncodeToString(h.Sum(nil))
		release.SizeBytes = n
	} else {
		// Standard flow: read from multipart file
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_FILE"}})
			return
		}
		defer src.Close()

		var reader io.Reader = src
		if requiresZipMagic(ext) {
			magic := make([]byte, 4)
			if n, _ := io.ReadFull(src, magic); n < 4 || !(magic[0] == 'P' && magic[1] == 'K') {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_FILE", "message": "invalid archive"}})
				return
			}
			reader = io.MultiReader(bytes.NewReader(magic), src)
		}

		dest, err := os.Create(destPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		defer dest.Close()
		h := sha256.New()
		if _, err := io.Copy(dest, io.TeeReader(reader, h)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": err.Error()}})
			return
		}
		release.SHA256 = hex.EncodeToString(h.Sum(nil))
		fi, _ := os.Stat(destPath)
		if fi != nil {
			release.SizeBytes = fi.Size()
		}
	}
	release.FileName = filename
	release.StoragePath = filepath.ToSlash(filepath.Join("uploads", p.ProductID, p.VariantID, release.ID, filename))

	// Check storage type from system settings
	var settings store.SystemSettings
	db.Where("id = ?", "default").First(&settings)
	storageType := settings.StorageType
	if storageType == "" {
		storageType = "local"
	}

	// For iOS: extract and save provisioning profile (before S3 upload, needs local file)
	if platform == "ios" {
		go func(releaseID, storagePath string) {
			saveProvisioningProfile(db, releaseID, storagePath)
		}(release.ID, destPath)
	}

	// Optional: save app icon if provided
	if iconB64 := p.IconBase64; iconB64 != "" {
		// strip data URL prefix
		if idx := strings.Index(iconB64, "base64,"); idx >= 0 {
			iconB64 = iconB64[idx+7:]
		}
		if data, err := base64.StdEncoding.DecodeString(iconB64); err == nil && len(data) > 0 {
			// ensure parent dir exists (uploads/{product.ID})
			_ = os.MkdirAll(filepath.Join("uploads", p.ProductID), 0o755)
			iconPath := filepath.ToSlash(filepath.Join("uploads", p.ProductID, "icon.png"))
			if writeErr := os.WriteFile(iconPath, data, 0o644); writeErr == nil {
				_ = db.Model(&store.Product{}).Where("id = ?", p.ProductID).Update("icon_path", iconPath).Error
				if p.LegacyAppID != "" {
					_ = db.Model(&store.App{}).Where("id = ?", p.LegacyAppID).Update("icon_path", iconPath).Error
				}
			}
		}
	}

	// If S3 mode, upload file to S3/R2
	if storageType == "s3" && settings.S3Endpoint != "" && settings.S3Bucket != "" && settings.S3AccessKey != "" && settings.S3SecretKey != "" {
		s3c := s3client.NewClient(settings.S3Endpoint, settings.S3Bucket, settings.S3AccessKey, settings.S3SecretKey)
		s3Key := fmt.Sprintf("uploads/%s/%s/%s/%s", p.ProductID, p.VariantID, release.ID, filename)

		f, err := os.Open(destPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "IO", "message": "failed to read file for S3 upload"}})
			return
		}
		if err := s3c.Upload(c.Request.Context(), s3Key, f, release.MimeType); err != nil {
			f.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "S3", "message": err.Error()}})
			return
		}
		f.Close()
		release.StoragePath = "s3://" + s3Key
	}

	if err := db.Create(&release).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
		return
	}
	_ = db.Create(&store.Event{Type: "upload", AppID: p.LegacyAppID, VariantID: p.VariantID, ReleaseID: release.ID, IP: c.ClientIP(), UA: c.Request.UserAgent()}).Error

	// meta.json snapshot (local storage only)
	if storageType == "local" {
		meta := map[string]any{
			"app_id": p.LegacyAppID, "product_id": p.ProductID, "variant_id": p.VariantID, "release_id": release.ID, "platform": platform,
			"identifier": p.Identifier, "version": version, "build": build,
			"min_os": minOS, "size": release.SizeBytes, "sha256": release.SHA256,
			"changelog": changelog, "uploaded_by": "api", "uploaded_at": time.Now().UTC(),
		}
		if mf, err := os.Create(filepath.Join(appDir, "meta.json")); err != nil {
			log.Printf("[upload] WARN: failed to write meta.json (variant=%s release=%s): %v", p.VariantID, release.ID, err)
		} else {
			_ = json.NewEncoder(mf).Encode(meta)
			_ = mf.Close()
		}
	}

	// Determine domains (prefer DB settings; fall back to config/request)
	primary := cfg.GetPrimaryDomain()
	if settings.PrimaryDomain != "" {
		primary = settings.PrimaryDomain
	}
	downloadBase := ""
	if settings.SecondaryDomains != "" {
		var secs []string
		_ = json.Unmarshal([]byte(settings.SecondaryDomains), &secs)
		if len(secs) > 0 && secs[0] != "" {
			downloadBase = secs[0]
		}
	}
	// Determine scheme
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" && c.Request.TLS != nil {
		scheme = "https"
	}
	if scheme == "" {
		scheme = "http"
	}
	// Normalize domain value to match request scheme and ensure protocol present
	normalize := func(raw string) string {
		r := strings.TrimSpace(raw)
		if r == "" {
			return scheme + "://" + c.Request.Host
		}
		// default/localhost should use current host
		if strings.Contains(r, "localhost") || strings.Contains(r, "127.0.0.1") {
			return scheme + "://" + c.Request.Host
		}
		if strings.HasPrefix(r, "http://") {
			if scheme == "https" {
				return "https://" + strings.TrimPrefix(r, "http://")
			}
			return r
		}
		if strings.HasPrefix(r, "https://") {
			return r
		}
		// no scheme provided
		return scheme + "://" + r
	}
	primary = normalize(primary)
	if downloadBase == "" {
		downloadBase = primary
	} else {
		downloadBase = normalize(downloadBase)
	}
	productPathKey := p.ProductSlug
	if productPathKey == "" {
		productPathKey = p.ProductID
	}
	manifestURL := fmt.Sprintf("%s/ios/%s/manifest.plist", primary, release.ID)
	urls := gin.H{
		"page":         fmt.Sprintf("%s/products/%s", primary, productPathKey),
		"release_page": fmt.Sprintf("%s/products/%s?r=%s&variant=%s", primary, productPathKey, release.ID, p.VariantID),
		"download":     fmt.Sprintf("%s/d/%s", downloadBase, release.ID),
		"ios_manifest": manifestURL,
		"ios_install":  "itms-services://?action=download-manifest&url=" + url.QueryEscape(manifestURL),
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "data": gin.H{"app": gin.H{"id": p.LegacyAppID, "name": p.ProductName, "platform": p.Platform, "bundle_id": p.Identifier, "product_id": p.ProductID, "variant_id": p.VariantID}, "release": gin.H{"id": release.ID, "version": release.Version, "build": release.Build, "created_at": release.CreatedAt}, "urls": urls}})
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func randHexN(n int) string {
	b := make([]byte, n)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}

type uploadTarget struct {
	Product     store.Product
	Variant     store.Variant
	LegacyAppID string
}

func loadUploadTarget(db *gorm.DB, variantID string) (uploadTarget, error) {
	var variant store.Variant
	if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uploadTarget{}, fmt.Errorf("variant not found")
		}
		return uploadTarget{}, err
	}

	var product store.Product
	if err := db.Where("id = ?", variant.ProductID).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uploadTarget{}, fmt.Errorf("product not found")
		}
		return uploadTarget{}, err
	}

	target := uploadTarget{Product: product, Variant: variant}
	if variant.LegacyAppID != nil {
		target.LegacyAppID = *variant.LegacyAppID
	}
	return target, nil
}

func detectFileExt(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, ext := range []string{".tar.gz", ".appimage", ".ipa", ".apk", ".dmg", ".pkg", ".exe", ".msi", ".deb", ".rpm", ".zip"} {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	return strings.ToLower(filepath.Ext(lower))
}

func isAllowedVariantFileExt(platform, ext string) bool {
	allowed := map[string]map[string]struct{}{
		"ios": {
			".ipa": {},
		},
		"android": {
			".apk": {},
		},
		"macos": {
			".dmg": {}, ".pkg": {}, ".zip": {},
		},
		"windows": {
			".exe": {}, ".msi": {}, ".zip": {},
		},
		"linux": {
			".appimage": {}, ".deb": {}, ".rpm": {}, ".tar.gz": {}, ".zip": {},
		},
	}
	set, ok := allowed[strings.ToLower(platform)]
	if !ok {
		return false
	}
	_, ok = set[strings.ToLower(ext)]
	return ok
}

func requiresZipMagic(ext string) bool {
	switch strings.ToLower(ext) {
	case ".ipa", ".apk", ".zip":
		return true
	default:
		return false
	}
}

func contentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".apk":
		return "application/vnd.android.package-archive"
	case ".ipa":
		return "application/octet-stream"
	case ".dmg":
		return "application/x-apple-diskimage"
	case ".pkg":
		return "application/octet-stream"
	case ".exe":
		return "application/vnd.microsoft.portable-executable"
	case ".msi":
		return "application/x-msi"
	case ".deb":
		return "application/vnd.debian.binary-package"
	case ".rpm":
		return "application/x-rpm"
	case ".appimage":
		return "application/octet-stream"
	case ".tar.gz":
		return "application/gzip"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func coalesceFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// saveProvisioningProfile extracts and saves provisioning profile from an IPA file
func saveProvisioningProfile(db *gorm.DB, releaseID, ipaPath string) {
	// Open IPA file (it's a zip archive)
	zr, err := zip.OpenReader(ipaPath)
	if err != nil {
		return
	}
	defer zr.Close()

	// Find Payload/*.app directory and embedded.mobileprovision
	var provisionData []byte
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "Payload/") && strings.HasSuffix(f.Name, ".app/embedded.mobileprovision") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			provisionData, _ = io.ReadAll(rc)
			rc.Close()
			break
		}
	}

	if provisionData == nil {
		return
	}

	// Extract plist from CMS signed data
	plistData := extractProvisionPlist(provisionData)
	if plistData == nil {
		return
	}

	// Parse plist
	var m map[string]interface{}
	if _, err := plist.Unmarshal(plistData, &m); err != nil {
		return
	}

	// Build ProvisioningProfile record
	profile := store.ProvisioningProfile{
		ID:        "prof_" + randHexN(8),
		ReleaseID: releaseID,
		CreatedAt: time.Now(),
	}

	// Extract fields
	if v, ok := m["UUID"].(string); ok {
		profile.UUID = v
	}
	if v, ok := m["Name"].(string); ok {
		profile.Name = v
	}
	if v, ok := m["AppIDName"].(string); ok {
		profile.AppIDName = v
	}
	if v, ok := m["TeamName"].(string); ok {
		profile.TeamName = v
	}

	// Team ID
	if teams, ok := m["TeamIdentifier"].([]interface{}); ok && len(teams) > 0 {
		if teamID, ok := teams[0].(string); ok {
			profile.TeamID = teamID
		}
	}

	// App ID prefix
	if prefixes, ok := m["ApplicationIdentifierPrefix"].([]interface{}); ok && len(prefixes) > 0 {
		if prefix, ok := prefixes[0].(string); ok {
			profile.AppIDPrefix = prefix
		}
	}

	// Dates
	if v, ok := m["CreationDate"].(time.Time); ok {
		profile.CreationDate = v
	}
	if v, ok := m["ExpirationDate"].(time.Time); ok {
		profile.ExpirationDate = v
	}

	// Platform
	if platforms, ok := m["Platform"].([]interface{}); ok && len(platforms) > 0 {
		platStrs := make([]string, 0, len(platforms))
		for _, p := range platforms {
			if ps, ok := p.(string); ok {
				platStrs = append(platStrs, ps)
			}
		}
		profile.Platform = strings.Join(platStrs, ", ")
	}

	// ProvisionsAllDevices
	if v, ok := m["ProvisionsAllDevices"].(bool); ok {
		profile.ProvisionsAllDevices = v
	}

	// Entitlements
	if ent, ok := m["Entitlements"].(map[string]interface{}); ok {
		if entJSON, err := json.Marshal(ent); err == nil {
			profile.Entitlements = string(entJSON)
		}
		// Extract bundle ID
		if appID, ok := ent["application-identifier"].(string); ok {
			if idx := strings.Index(appID, "."); idx > 0 {
				profile.BundleID = appID[idx+1:]
			}
		}
	}

	// Provisioned devices
	if devices, ok := m["ProvisionedDevices"].([]interface{}); ok {
		deviceList := make([]string, 0, len(devices))
		for _, d := range devices {
			if udid, ok := d.(string); ok {
				deviceList = append(deviceList, udid)
			}
		}
		if devJSON, err := json.Marshal(deviceList); err == nil {
			profile.ProvisionedDevices = string(devJSON)
		}
	}

	// Determine profile type
	profile.ProfileType = determineProvisionProfileType(m)

	// Parse certificates
	if certs, ok := m["DeveloperCertificates"].([]interface{}); ok {
		certInfos := parseProvisionCertificates(certs)
		if certJSON, err := json.Marshal(certInfos); err == nil {
			profile.Certificates = string(certJSON)
		}
	}

	// Save to database
	_ = db.Create(&profile).Error
}

// extractProvisionPlist extracts plist XML from CMS-signed mobileprovision
func extractProvisionPlist(data []byte) []byte {
	startMarker := []byte("<?xml")
	endMarker := []byte("</plist>")

	startIdx := bytes.Index(data, startMarker)
	if startIdx < 0 {
		return nil
	}

	endIdx := bytes.Index(data[startIdx:], endMarker)
	if endIdx < 0 {
		return nil
	}

	return data[startIdx : startIdx+endIdx+len(endMarker)]
}

// determineProvisionProfileType determines the profile type
func determineProvisionProfileType(m map[string]interface{}) string {
	if v, ok := m["ProvisionsAllDevices"].(bool); ok && v {
		return "enterprise"
	}

	if ent, ok := m["Entitlements"].(map[string]interface{}); ok {
		if getTaskAllow, ok := ent["get-task-allow"].(bool); ok && getTaskAllow {
			return "development"
		}
		if _, ok := ent["beta-reports-active"]; ok {
			return "app-store"
		}
	}

	if devices, ok := m["ProvisionedDevices"].([]interface{}); ok && len(devices) > 0 {
		return "ad-hoc"
	}

	return "distribution"
}

// ProvisionCertInfo holds certificate info for JSON storage
type ProvisionCertInfo struct {
	Name         string `json:"name"`
	SerialNumber string `json:"serial_number"`
	SHA1         string `json:"sha1"`
	CreationDate string `json:"creation_date"`
	ExpiryDate   string `json:"expiry_date"`
}

// parseProvisionCertificates parses certificates from profile
func parseProvisionCertificates(certs []interface{}) []ProvisionCertInfo {
	var result []ProvisionCertInfo

	for _, c := range certs {
		certData, ok := c.([]byte)
		if !ok {
			continue
		}

		cert, err := x509.ParseCertificate(certData)
		if err != nil {
			continue
		}

		h := sha1.Sum(certData)

		info := ProvisionCertInfo{
			Name:         cert.Subject.CommonName,
			SerialNumber: cert.SerialNumber.Text(16),
			SHA1:         fmt.Sprintf("%X", h),
			CreationDate: cert.NotBefore.Format(time.RFC3339),
			ExpiryDate:   cert.NotAfter.Format(time.RFC3339),
		}

		result = append(result, info)
	}

	return result
}
