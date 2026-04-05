package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
)

func openRuntimeTestDB(t *testing.T) *gorm.DB {
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

func seedRuntimeVariantFixture(t *testing.T, db *gorm.DB, platform string) (store.Product, store.Variant, store.Release) {
	t.Helper()

	product := store.Product{
		ID:        "prd_runtime_" + platform,
		Slug:      "runtime-" + platform,
		Name:      "Runtime " + strings.ToUpper(platform),
		Published: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	variant := store.Variant{
		ID:            "var_runtime_" + platform,
		ProductID:     product.ID,
		Platform:      platform,
		Identifier:    "com.example." + platform,
		DisplayName:   product.Name,
		InstallerType: map[string]string{"ios": "ipa", "windows": "msi"}[platform],
		Published:     true,
	}
	if err := db.Create(&variant).Error; err != nil {
		t.Fatalf("create variant: %v", err)
	}

	ext := ".bin"
	switch platform {
	case "ios":
		ext = ".ipa"
	case "windows":
		ext = ".msi"
	}

	release := store.Release{
		ID:          "rel_runtime_" + platform,
		VariantID:   variant.ID,
		Version:     "1.0.0",
		Build:       1,
		FileExt:     ext,
		StoragePath: filepath.ToSlash(filepath.Join("uploads", product.ID, variant.ID, "rel_runtime_"+platform, "app"+ext)),
		CreatedAt:   time.Now(),
	}
	if err := db.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}

	return product, variant, release
}

func TestDownloadUsesVariantProductWhenLegacyAppIDIsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openRuntimeTestDB(t)
	_, _, release := seedRuntimeVariantFixture(t, db, "windows")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	fullPath := filepath.Join(tmp, release.StoragePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("runtime"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/d/"+release.ID, nil)
	c.Params = gin.Params{{Key: "releaseID", Value: release.ID}}

	Download(db)(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var event store.Event
	if err := db.Where("type = ? AND release_id = ?", "download", release.ID).Take(&event).Error; err != nil {
		t.Fatalf("load download event: %v", err)
	}
	if event.VariantID != release.VariantID {
		t.Fatalf("expected download event variant_id %q, got %q", release.VariantID, event.VariantID)
	}
}

func TestDownloadDoesNotCountMissingFileAsDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openRuntimeTestDB(t)
	_, _, release := seedRuntimeVariantFixture(t, db, "windows")

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/d/"+release.ID, nil)
	c.Params = gin.Params{{Key: "releaseID", Value: release.ID}}

	Download(db)(c)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d body=%s", rr.Code, rr.Body.String())
	}

	var refreshed store.Release
	if err := db.Where("id = ?", release.ID).First(&refreshed).Error; err != nil {
		t.Fatalf("reload release: %v", err)
	}
	if refreshed.DownloadCount != 0 {
		t.Fatalf("expected download_count to remain 0, got %d", refreshed.DownloadCount)
	}

	var count int64
	if err := db.Model(&store.Event{}).Where("type = ? AND release_id = ?", "download", release.ID).Count(&count).Error; err != nil {
		t.Fatalf("count download events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no download events for missing file, got %d", count)
	}
}

func TestIOSManifestUsesVariantIdentifierAndProductName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openRuntimeTestDB(t)
	cfg := config.Default()
	cfg.Server.PrimaryDomain = "https://download.example.com"
	product, variant, release := seedRuntimeVariantFixture(t, db, "ios")

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	req := httptest.NewRequest(http.MethodGet, "/ios/"+release.ID+"/manifest.plist", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)")
	c.Request = req
	c.Params = gin.Params{{Key: "releaseID", Value: release.ID}}

	IOSManifest(db, cfg)(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), variant.Identifier) {
		t.Fatalf("expected manifest to include variant identifier, body=%s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), product.Name) {
		t.Fatalf("expected manifest to include product name, body=%s", rr.Body.String())
	}
}

func TestUDIDCallbackBindsDeviceToVariantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openRuntimeTestDB(t)
	cfg := config.Default()
	cfg.Server.PrimaryDomain = "https://download.example.com"
	product, variant, _ := seedRuntimeVariantFixture(t, db, "ios")

	nonce := store.UDIDNonce{
		Nonce:     "nonce_variant",
		VariantID: variant.ID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := db.Create(&nonce).Error; err != nil {
		t.Fatalf("create nonce: %v", err)
	}

	body := `<?xml version="1.0"?><plist><dict><key>UDID</key><string>ABC-123</string><key>PRODUCT</key><string>iPhone15,3</string><key>VERSION</key><string>17.4</string><key>DEVICE_NAME</key><string>Test iPhone</string></dict></plist>`
	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	req := httptest.NewRequest(http.MethodPost, "/udid/callback?nonce="+nonce.Nonce, strings.NewReader(body))
	req.Header.Set("User-Agent", "Profile Service")
	req.Header.Set("X-Forwarded-Proto", "https")
	c.Request = req

	UDIDCallback(db, cfg)(c)

	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("expected status 301, got %d body=%s", rr.Code, rr.Body.String())
	}

	var binding store.DeviceAppBinding
	if err := db.Where("variant_id = ?", variant.ID).Take(&binding).Error; err != nil {
		t.Fatalf("expected variant binding: %v", err)
	}
	if binding.VariantID != variant.ID {
		t.Fatalf("expected binding variant_id %q, got %q", variant.ID, binding.VariantID)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/products/"+product.Slug) {
		t.Fatalf("expected redirect to product page, got %q", location)
	}
	if !strings.Contains(location, "variant="+variant.ID) {
		t.Fatalf("expected redirect to include variant id, got %q", location)
	}
}

func TestUDIDStatusScopesUsedNonceToVariant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openRuntimeTestDB(t)
	_, variant, _ := seedRuntimeVariantFixture(t, db, "ios")
	_, otherVariant, _ := seedRuntimeVariantFixture(t, db, "windows")
	otherVariant.ID = "var_runtime_ios_other"
	otherVariant.Platform = "ios"
	otherVariant.Identifier = "com.example.ios.other"
	otherVariant.ProductID = variant.ProductID
	otherVariant.InstallerType = "ipa"
	if err := db.Create(&otherVariant).Error; err != nil {
		t.Fatalf("create other variant: %v", err)
	}

	nonce := store.UDIDNonce{
		Nonce:     "nonce_scope",
		VariantID: variant.ID,
		Used:      true,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := db.Create(&nonce).Error; err != nil {
		t.Fatalf("create nonce: %v", err)
	}

	assertStatus := func(targetVariantID string, expected bool) {
		t.Helper()
		rr := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rr)
		c.Request = httptest.NewRequest(http.MethodGet, "/udid/status?nonce="+nonce.Nonce+"&variant="+targetVariantID, nil)

		UDIDStatus(db)(c)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		if expected && !strings.Contains(rr.Body.String(), `"bound":true`) {
			t.Fatalf("expected bound=true for variant %q, body=%s", targetVariantID, rr.Body.String())
		}
		if !expected && !strings.Contains(rr.Body.String(), `"bound":false`) {
			t.Fatalf("expected bound=false for variant %q, body=%s", targetVariantID, rr.Body.String())
		}
	}

	assertStatus(variant.ID, true)
	assertStatus(otherVariant.ID, false)
}
