package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

func openAdminProductsTestDB(t *testing.T) *gorm.DB {
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

func seedAdminProduct(t *testing.T, db *gorm.DB) (store.Product, store.Variant) {
	t.Helper()

	product := store.Product{
		ID:        "prd_admin",
		Slug:      "admin-suite",
		Name:      "Admin Suite",
		Published: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	variant := store.Variant{
		ID:            "var_admin_ios",
		ProductID:     product.ID,
		Platform:      "ios",
		Identifier:    "com.example.admin",
		DisplayName:   "Admin iOS",
		InstallerType: "ipa",
		Published:     true,
	}
	if err := db.Create(&variant).Error; err != nil {
		t.Fatalf("create variant: %v", err)
	}

	release := store.Release{
		ID:        "rel_admin_ios",
		VariantID: variant.ID,
		Version:   "1.0.0",
		Build:     1,
		CreatedAt: time.Now(),
	}
	if err := db.Create(&release).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}

	return product, variant
}

func performAdminJSONRequest(t *testing.T, handler gin.HandlerFunc, method, path string, params gin.Params, body any) map[string]any {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(b)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = params

	handler(c)

	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("expected success status, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func TestAdminListProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)
	product, _ := seedAdminProduct(t, db)

	payload := performAdminJSONRequest(t, AdminListProducts(db), http.MethodGet, "/admin/api/products", nil, nil)
	data := payload["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 product, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["id"] != product.ID {
		t.Fatalf("expected product id %q, got %v", product.ID, item["id"])
	}
}

func TestAdminGetProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)
	product, variant := seedAdminProduct(t, db)

	payload := performAdminJSONRequest(t, AdminGetProduct(db), http.MethodGet, "/admin/api/products/"+product.ID, gin.Params{
		{Key: "productID", Value: product.ID},
	}, nil)

	data := payload["data"].(map[string]any)
	productData := data["product"].(map[string]any)
	if productData["id"] != product.ID {
		t.Fatalf("expected product id %q, got %v", product.ID, productData["id"])
	}
	variants := data["variants"].([]any)
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	item := variants[0].(map[string]any)
	if item["id"] != variant.ID {
		t.Fatalf("expected variant id %q, got %v", variant.ID, item["id"])
	}
}

func TestAdminUpsertVariant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)
	product, variant := seedAdminProduct(t, db)

	createPayload := performAdminJSONRequest(t, AdminCreateVariant(db), http.MethodPost, "/admin/api/products/"+product.ID+"/variants", gin.Params{
		{Key: "productID", Value: product.ID},
	}, map[string]any{
		"platform":       "windows",
		"identifier":     "com.example.windows",
		"display_name":   "Admin Windows",
		"installer_type": "msi",
	})
	created := createPayload["data"].(map[string]any)["variant"].(map[string]any)
	if created["platform"] != "windows" {
		t.Fatalf("expected created variant platform windows, got %v", created["platform"])
	}

	updatePayload := performAdminJSONRequest(t, AdminUpdateVariant(db), http.MethodPut, "/admin/api/variants/"+variant.ID, gin.Params{
		{Key: "variantID", Value: variant.ID},
	}, map[string]any{
		"display_name": "Updated Admin iOS",
		"published":    false,
	})
	updated := updatePayload["data"].(map[string]any)["variant"].(map[string]any)
	if updated["display_name"] != "Updated Admin iOS" {
		t.Fatalf("expected updated display name, got %v", updated["display_name"])
	}
	if updated["published"] != false {
		t.Fatalf("expected variant to be unpublished, got %v", updated["published"])
	}
}

func TestAdminCreateAndUpdateProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)

	createPayload := performAdminJSONRequest(t, AdminCreateProduct(db), http.MethodPost, "/admin/api/products", nil, map[string]any{
		"name":        "Desktop Suite",
		"slug":        "desktop-suite",
		"description": "Unified desktop downloads",
		"published":   false,
	})

	created := createPayload["data"].(map[string]any)["product"].(map[string]any)
	if created["name"] != "Desktop Suite" {
		t.Fatalf("expected created product name, got %v", created["name"])
	}
	if created["published"] != false {
		t.Fatalf("expected created product published=false, got %v", created["published"])
	}

	productID := created["id"].(string)
	updatePayload := performAdminJSONRequest(t, AdminUpdateProduct(db), http.MethodPut, "/admin/api/products/"+productID, gin.Params{
		{Key: "productID", Value: productID},
	}, map[string]any{
		"name":      "Desktop Suite Updated",
		"published": true,
	})

	updated := updatePayload["data"].(map[string]any)["product"].(map[string]any)
	if updated["name"] != "Desktop Suite Updated" {
		t.Fatalf("expected updated product name, got %v", updated["name"])
	}
	if updated["published"] != true {
		t.Fatalf("expected updated product published=true, got %v", updated["published"])
	}
}

func TestAdminGetVariantStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)
	product, variant := seedAdminProduct(t, db)
	if err := db.Create(&store.Release{
		ID:            "rel_admin_ios_2",
		VariantID:     variant.ID,
		Version:       "1.1.0",
		Build:         2,
		DownloadCount: 88,
		Channel:       "beta",
		CreatedAt:     time.Now().Add(time.Minute),
	}).Error; err != nil {
		t.Fatalf("create second release: %v", err)
	}

	payload := performAdminJSONRequest(t, AdminGetVariantStats(db), http.MethodGet, "/admin/api/variants/"+variant.ID+"/stats", gin.Params{
		{Key: "variantID", Value: variant.ID},
	}, nil)

	data := payload["data"].(map[string]any)
	productData := data["product"].(map[string]any)
	if productData["id"] != product.ID {
		t.Fatalf("expected product id %q, got %v", product.ID, productData["id"])
	}

	variantData := data["variant"].(map[string]any)
	if variantData["id"] != variant.ID {
		t.Fatalf("expected variant id %q, got %v", variant.ID, variantData["id"])
	}

	releases := data["releases"].([]any)
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	first := releases[0].(map[string]any)
	if first["version"] != "1.1.0" {
		t.Fatalf("expected latest release first, got %v", first["version"])
	}
	if first["download_count"] != float64(88) {
		t.Fatalf("expected download count 88, got %v", first["download_count"])
	}
}
