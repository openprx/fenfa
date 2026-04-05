package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	"github.com/openprx/fenfa/internal/store"
)

func openProductsAPITestDB(t *testing.T) *gorm.DB {
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

func seedProductFixture(t *testing.T, db *gorm.DB) (store.Product, store.Variant, store.Variant) {
	t.Helper()

	product := store.Product{
		ID:        "prd_suite",
		Slug:      "demo-suite",
		Name:      "Demo Suite",
		Published: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	iosVariant := store.Variant{
		ID:            "var_ios",
		ProductID:     product.ID,
		Platform:      "ios",
		Identifier:    "com.example.ios",
		DisplayName:   "Demo iOS",
		InstallerType: "ipa",
		Published:     true,
	}
	if err := db.Create(&iosVariant).Error; err != nil {
		t.Fatalf("create ios variant: %v", err)
	}

	windowsVariant := store.Variant{
		ID:            "var_windows",
		ProductID:     product.ID,
		Platform:      "windows",
		Identifier:    "com.example.windows",
		DisplayName:   "Demo Windows",
		InstallerType: "msi",
		Published:     true,
	}
	if err := db.Create(&windowsVariant).Error; err != nil {
		t.Fatalf("create windows variant: %v", err)
	}

	iosRelease := store.Release{
		ID:          "rel_ios",
		VariantID:   iosVariant.ID,
		Version:     "1.2.0",
		Build:       12,
		Changelog:   "ios changelog",
		StoragePath: "uploads/prd_suite/var_ios/rel_ios/app.ipa",
		CreatedAt:   time.Now(),
	}
	if err := db.Create(&iosRelease).Error; err != nil {
		t.Fatalf("create ios release: %v", err)
	}

	windowsRelease := store.Release{
		ID:          "rel_windows",
		VariantID:   windowsVariant.ID,
		Version:     "2.5.0",
		Build:       25,
		Changelog:   "windows changelog",
		StoragePath: "uploads/prd_suite/var_windows/rel_windows/app.msi",
		FileExt:     ".msi",
		CreatedAt:   time.Now().Add(time.Minute),
	}
	if err := db.Create(&windowsRelease).Error; err != nil {
		t.Fatalf("create windows release: %v", err)
	}

	profile := store.ProvisioningProfile{
		ID:         "prof_ios",
		ReleaseID:  iosRelease.ID,
		UUID:       "uuid-ios",
		Name:       "iOS Profile",
		Platform:   "iOS",
		ProfileType: "ad-hoc",
		CreatedAt:  time.Now(),
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create provisioning profile: %v", err)
	}

	return product, iosVariant, windowsVariant
}

func performJSONRequest(t *testing.T, handler gin.HandlerFunc, method, path string, params gin.Params) map[string]any {
	t.Helper()

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "download.example.com"
	c.Request = req
	c.Params = params

	handler(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func TestProductDetailJSONReturnsMultiPlatformVariants(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openProductsAPITestDB(t)
	cfg := config.Default()
	cfg.Server.PrimaryDomain = "https://download.example.com"
	product, _, _ := seedProductFixture(t, db)
	legacyIOSAppID := "app_legacy_ios"
	if err := db.Model(&store.Variant{}).Where("id = ?", "var_ios").Update("legacy_app_id", legacyIOSAppID).Error; err != nil {
		t.Fatalf("set variant legacy app id: %v", err)
	}

	payload := performJSONRequest(t, ProductDetailJSON(db, cfg), http.MethodGet, "/api/products/"+product.ID, gin.Params{
		{Key: "productID", Value: product.ID},
	})

	data := payload["data"].(map[string]any)
	productData := data["product"].(map[string]any)
	if productData["id"] != product.ID {
		t.Fatalf("expected product id %q, got %v", product.ID, productData["id"])
	}

	variants := data["variants"].([]any)
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(variants))
	}

	var iosVariant map[string]any
	var windowsVariant map[string]any
	for _, item := range variants {
		variant := item.(map[string]any)
		switch variant["platform"] {
		case "ios":
			iosVariant = variant
		case "windows":
			windowsVariant = variant
		}
	}

	if iosVariant == nil || windowsVariant == nil {
		t.Fatalf("expected ios and windows variants in payload")
	}
	if iosVariant["legacy_app_id"] != legacyIOSAppID {
		t.Fatalf("expected ios legacy_app_id %q, got %v", legacyIOSAppID, iosVariant["legacy_app_id"])
	}

	iosLatest := iosVariant["latest_release"].(map[string]any)
	iosActions := iosLatest["actions"].(map[string]any)
	if iosActions["ios_install"] == "" {
		t.Fatalf("expected ios install action for ios variant")
	}
	if iosLatest["changelog"] != "ios changelog" {
		t.Fatalf("expected ios changelog, got %v", iosLatest["changelog"])
	}

	windowsLatest := windowsVariant["latest_release"].(map[string]any)
	windowsActions := windowsLatest["actions"].(map[string]any)
	if windowsActions["download"] == "" {
		t.Fatalf("expected download action for windows variant")
	}
	if _, ok := windowsActions["ios_install"]; ok {
		t.Fatalf("did not expect ios_install action for windows variant")
	}
	if windowsLatest["changelog"] != "windows changelog" {
		t.Fatalf("expected windows changelog, got %v", windowsLatest["changelog"])
	}
}

func TestProductDetailJSONUsesProductIconRouteWithoutLegacyApp(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openProductsAPITestDB(t)
	cfg := config.Default()
	cfg.Server.PrimaryDomain = "https://download.example.com"
	product, _, _ := seedProductFixture(t, db)
	if err := db.Model(&store.Product{}).Where("id = ?", product.ID).Update("icon_path", "uploads/prd_suite/icon.png").Error; err != nil {
		t.Fatalf("set product icon path: %v", err)
	}

	payload := performJSONRequest(t, ProductDetailJSON(db, cfg), http.MethodGet, "/api/products/"+product.ID, gin.Params{
		{Key: "productID", Value: product.ID},
	})

	data := payload["data"].(map[string]any)
	productData := data["product"].(map[string]any)
	if productData["icon_url"] != "https://download.example.com/icon/products/"+product.ID {
		t.Fatalf("expected product icon url, got %v", productData["icon_url"])
	}
}

func TestLegacyAppDetailJSONUsesMigratedVariantData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openProductsAPITestDB(t)
	cfg := config.Default()
	cfg.Server.PrimaryDomain = "https://download.example.com"
	product, iosVariant, _ := seedProductFixture(t, db)

	legacyID := "app_legacy_ios"
	legacyApp := store.App{
		ID:        legacyID,
		Platform:  "ios",
		BundleID:  iosVariant.Identifier,
		Name:      product.Name,
		Published: true,
	}
	if err := db.Create(&legacyApp).Error; err != nil {
		t.Fatalf("create legacy app: %v", err)
	}
	if err := db.Model(&store.Product{}).Where("id = ?", product.ID).Update("legacy_app_id", legacyID).Error; err != nil {
		t.Fatalf("link product legacy id: %v", err)
	}
	if err := db.Model(&store.Variant{}).Where("id = ?", iosVariant.ID).Update("legacy_app_id", legacyID).Error; err != nil {
		t.Fatalf("link variant legacy id: %v", err)
	}
	if err := db.Model(&store.Release{}).Where("variant_id = ?", iosVariant.ID).Update("app_id", legacyID).Error; err != nil {
		t.Fatalf("link release app id: %v", err)
	}

	payload := performJSONRequest(t, AppDetailJSON(db, cfg), http.MethodGet, "/api/apps/"+legacyID, gin.Params{
		{Key: "appID", Value: legacyID},
	})

	data := payload["data"].(map[string]any)
	appData := data["app"].(map[string]any)
	if appData["id"] != legacyID {
		t.Fatalf("expected legacy app id %q, got %v", legacyID, appData["id"])
	}

	releases := data["releases"].([]any)
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}

	release := releases[0].(map[string]any)
	if release["changelog"] != "ios changelog" {
		t.Fatalf("expected legacy app changelog from migrated variant, got %v", release["changelog"])
	}
}

func TestLegacyAppRedirectsToProductPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openProductsAPITestDB(t)
	product, iosVariant, _ := seedProductFixture(t, db)
	legacyID := "app_legacy_redirect"
	if err := db.Model(&store.Variant{}).Where("id = ?", iosVariant.ID).Update("legacy_app_id", legacyID).Error; err != nil {
		t.Fatalf("link variant legacy id: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/apps/"+legacyID+"?r=rel_ios", nil)
	c.Params = gin.Params{{Key: "appID", Value: legacyID}}

	LegacyAppRedirect(db)(c)

	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("expected status 301, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/products/"+product.Slug+"?r=rel_ios" {
		t.Fatalf("expected redirect to product page, got %q", got)
	}
}
