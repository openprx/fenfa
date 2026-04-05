package handlers

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/openprx/fenfa/internal/store"
)

func TestExportReleasesFiltersByVariantIDAndIncludesVariantColumn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)
	_, variant := seedAdminProduct(t, db)
	otherVariant := store.Variant{
		ID:            "var_admin_other",
		ProductID:     variant.ProductID,
		Platform:      "ios",
		Identifier:    "com.example.other",
		DisplayName:   "Other iOS",
		InstallerType: "ipa",
		Published:     true,
	}
	if err := db.Create(&otherVariant).Error; err != nil {
		t.Fatalf("create other variant: %v", err)
	}
	if err := db.Create(&store.Release{ID: "rel_filter_a", VariantID: variant.ID, Version: "1.0.1", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create release a: %v", err)
	}
	if err := db.Create(&store.Release{ID: "rel_filter_b", VariantID: otherVariant.ID, Version: "1.0.2", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create release b: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/exports/releases.csv?variant_id="+variant.ID, nil)

	ExportReleases(db)(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	rows, err := csv.NewReader(strings.NewReader(rr.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least one data row, got %v", rows)
	}
	if rows[0][1] != "variant_id" {
		t.Fatalf("expected variant_id column in header, got %v", rows[0])
	}
	for _, row := range rows[1:] {
		if row[1] != variant.ID {
			t.Fatalf("expected only variant %q rows, got %v", variant.ID, row)
		}
	}
}

func TestExportEventsFiltersByVariantIDAndIncludesVariantColumn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openEventsTestDB(t)
	if err := db.Create(&[]store.Event{
		{Type: "download", VariantID: "var_a", ReleaseID: "rel_a", Ts: time.Now()},
		{Type: "download", VariantID: "var_b", ReleaseID: "rel_b", Ts: time.Now()},
	}).Error; err != nil {
		t.Fatalf("create events: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/exports/events.csv?variant_id=var_b", nil)

	ExportEvents(db)(c)

	rows, err := csv.NewReader(strings.NewReader(rr.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected exactly one exported event, got %v", rows)
	}
	if rows[0][3] != "variant_id" {
		t.Fatalf("expected variant_id column in header, got %v", rows[0])
	}
	if rows[1][3] != "var_b" {
		t.Fatalf("expected exported variant_id var_b, got %v", rows[1])
	}
}

func TestAdminListIOSDevicesFiltersByVariantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)
	_, variant := seedAdminProduct(t, db)
	if err := db.Create(&store.IOSDevice{ID: "dev_a", UDID: "udid-a", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create device a: %v", err)
	}
	if err := db.Create(&store.IOSDevice{ID: "dev_b", UDID: "udid-b", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create device b: %v", err)
	}
	if err := db.Create(&store.DeviceAppBinding{ID: "bind_a", DeviceID: "dev_a", UDID: "udid-a", VariantID: variant.ID, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create binding a: %v", err)
	}
	if err := db.Create(&store.DeviceAppBinding{ID: "bind_b", DeviceID: "dev_b", UDID: "udid-b", VariantID: "var_other", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create binding b: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/api/ios_devices?variant_id="+variant.ID, nil)

	AdminListIOSDevices(db)(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items := payload["data"].(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 device, got %d", len(items))
	}
	if items[0].(map[string]any)["id"] != "dev_a" {
		t.Fatalf("expected device dev_a, got %v", items[0])
	}
}

func TestAdminListIOSVariantsReturnsVariantCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAdminProductsTestDB(t)
	product, variant := seedAdminProduct(t, db)

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/api/ios_variants", nil)

	AdminListIOSVariants(db)(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items := payload["data"].(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 ios variant, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["id"] != variant.ID {
		t.Fatalf("expected variant id %q, got %v", variant.ID, item["id"])
	}
	if item["product_name"] != product.Name {
		t.Fatalf("expected product name %q, got %v", product.Name, item["product_name"])
	}
}
