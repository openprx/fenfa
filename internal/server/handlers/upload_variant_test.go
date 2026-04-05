package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
)

func openHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := store.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func withUploadWorkdir(t *testing.T) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
}

func seedVariant(t *testing.T, db *gorm.DB, platform, identifier string) store.Variant {
	t.Helper()

	product := store.Product{
		ID:        "prd_" + strings.ReplaceAll(platform, " ", "_"),
		Slug:      platform + "-product",
		Name:      strings.ToUpper(platform) + " Product",
		Published: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	variant := store.Variant{
		ID:            "var_" + strings.ReplaceAll(platform, " ", "_"),
		ProductID:     product.ID,
		Platform:      platform,
		Identifier:    identifier,
		DisplayName:   product.Name,
		InstallerType: strings.TrimPrefix(strings.ToLower(filepath.Ext(identifier)), "."),
		Published:     true,
	}
	if err := db.Create(&variant).Error; err != nil {
		t.Fatalf("create variant: %v", err)
	}
	return variant
}

func makeMultipartRequest(t *testing.T, target string, fields map[string]string, filename string, content []byte) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	part, err := writer.CreateFormFile("app_file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v body=%s", err, rr.Body.String())
	}
	return payload
}

func TestUploadRequiresVariantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withUploadWorkdir(t)

	db := openHandlerTestDB(t)
	cfg := config.Default()
	req := makeMultipartRequest(t, "/upload", map[string]string{
		"version": "1.0.0",
		"build":   "1",
	}, "demo.apk", []byte("PK\x03\x04demo"))

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	Upload(db, cfg)(c)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	payload := decodeJSON(t, rr)
	errObj := payload["error"].(map[string]any)
	if errObj["message"] != "missing variant_id" {
		t.Fatalf("expected missing variant_id error, got %v", errObj["message"])
	}
}

func TestUploadRejectsMismatchedExtensionForVariantPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withUploadWorkdir(t)

	db := openHandlerTestDB(t)
	cfg := config.Default()
	variant := seedVariant(t, db, "windows", "com.example.windows")
	req := makeMultipartRequest(t, "/upload", map[string]string{
		"variant_id": variant.ID,
		"version":    "1.0.0",
		"build":      "1",
	}, "demo.apk", []byte("PK\x03\x04demo"))

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	Upload(db, cfg)(c)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	payload := decodeJSON(t, rr)
	errObj := payload["error"].(map[string]any)
	if errObj["message"] != "file extension mismatch" {
		t.Fatalf("expected file extension mismatch error, got %v", errObj["message"])
	}
}

func TestUploadAcceptsDesktopVariantFileAndPersistsRelease(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withUploadWorkdir(t)

	db := openHandlerTestDB(t)
	cfg := config.Default()
	variant := seedVariant(t, db, "windows", "com.example.windows")
	req := makeMultipartRequest(t, "/upload", map[string]string{
		"variant_id": variant.ID,
		"version":    "2.3.4",
		"build":      "23",
		"changelog":  "desktop update",
	}, "demo.msi", []byte("MSI-DEMO"))

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	Upload(db, cfg)(c)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var release store.Release
	if err := db.Where("variant_id = ?", variant.ID).Take(&release).Error; err != nil {
		t.Fatalf("load release: %v", err)
	}
	if release.Version != "2.3.4" {
		t.Fatalf("expected version 2.3.4, got %q", release.Version)
	}
	if release.FileExt != ".msi" {
		t.Fatalf("expected file_ext .msi, got %q", release.FileExt)
	}
	if !strings.Contains(release.StoragePath, variant.ID) {
		t.Fatalf("expected storage path to contain variant id, got %q", release.StoragePath)
	}
	var uploadEvent store.Event
	if err := db.Where("type = ? AND release_id = ?", "upload", release.ID).Take(&uploadEvent).Error; err != nil {
		t.Fatalf("load upload event: %v", err)
	}
	if uploadEvent.VariantID != variant.ID {
		t.Fatalf("expected upload event variant_id %q, got %q", variant.ID, uploadEvent.VariantID)
	}
}

func TestSmartUploadAcceptsDesktopVariantWithoutParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withUploadWorkdir(t)

	db := openHandlerTestDB(t)
	cfg := config.Default()
	variant := seedVariant(t, db, "linux", "com.example.linux")
	req := makeMultipartRequest(t, "/smart-upload", map[string]string{
		"variant_id": variant.ID,
		"version":    "9.9.9",
		"build":      "42",
		"changelog":  "linux release",
	}, "demo.AppImage", []byte("#!/bin/sh\necho demo\n"))

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	SmartUpload(db, cfg)(c)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var release store.Release
	if err := db.Where("variant_id = ?", variant.ID).Take(&release).Error; err != nil {
		t.Fatalf("load release: %v", err)
	}
	if release.Version != "9.9.9" {
		t.Fatalf("expected version 9.9.9, got %q", release.Version)
	}
	if release.Changelog != "linux release" {
		t.Fatalf("expected changelog to be preserved, got %q", release.Changelog)
	}
}
