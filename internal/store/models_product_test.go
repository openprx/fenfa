package store

import (
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func TestAutoMigrateAddsProductVariantSchema(t *testing.T) {
	db := openTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	if !db.Migrator().HasTable("products") {
		t.Fatalf("expected products table to exist")
	}
	if !db.Migrator().HasTable("variants") {
		t.Fatalf("expected variants table to exist")
	}
	if !db.Migrator().HasColumn(&Release{}, "variant_id") {
		t.Fatalf("expected releases.variant_id column to exist")
	}
	if !db.Migrator().HasColumn(&DeviceAppBinding{}, "variant_id") {
		t.Fatalf("expected device_app_bindings.variant_id column to exist")
	}
	if !db.Migrator().HasColumn(&UDIDNonce{}, "variant_id") {
		t.Fatalf("expected udid_nonces.variant_id column to exist")
	}
}

func TestAutoMigrateMigratesLegacyAppToProductVariant(t *testing.T) {
	db := openTestDB(t)

	if err := db.AutoMigrate(&App{}, &Release{}, &DeviceAppBinding{}, &UDIDNonce{}); err != nil {
		t.Fatalf("legacy migrate: %v", err)
	}

	legacyApp := App{
		ID:        "app_legacy",
		Platform:  "ios",
		BundleID:  "com.example.legacy",
		Name:      "Legacy App",
		Published: true,
	}
	if err := db.Create(&legacyApp).Error; err != nil {
		t.Fatalf("create legacy app: %v", err)
	}

	legacyRelease := Release{
		ID:      "rel_legacy",
		AppID:   legacyApp.ID,
		Version: "1.0.0",
		Build:   1,
	}
	if err := db.Create(&legacyRelease).Error; err != nil {
		t.Fatalf("create legacy release: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	type productRow struct {
		ID   string
		Slug string
		Name string
	}
	var product productRow
	if err := db.Table("products").Where("name = ?", legacyApp.Name).Take(&product).Error; err != nil {
		t.Fatalf("expected migrated product: %v", err)
	}
	if product.Slug == "" {
		t.Fatalf("expected migrated product slug")
	}

	type variantRow struct {
		ID         string
		ProductID  string
		Platform   string
		Identifier string
	}
	var variant variantRow
	if err := db.Table("variants").Where("product_id = ?", product.ID).Take(&variant).Error; err != nil {
		t.Fatalf("expected migrated variant: %v", err)
	}
	if variant.Platform != legacyApp.Platform {
		t.Fatalf("expected migrated platform %q, got %q", legacyApp.Platform, variant.Platform)
	}
	if variant.Identifier != legacyApp.BundleID {
		t.Fatalf("expected migrated identifier %q, got %q", legacyApp.BundleID, variant.Identifier)
	}

	var migratedRelease Release
	if err := db.Where("id = ?", legacyRelease.ID).Take(&migratedRelease).Error; err != nil {
		t.Fatalf("load migrated release: %v", err)
	}
	if migratedRelease.VariantID != variant.ID {
		t.Fatalf("expected release variant_id %q, got %q", variant.ID, migratedRelease.VariantID)
	}
}

func TestAutoMigrateIsIdempotentForLegacyAppMigration(t *testing.T) {
	db := openTestDB(t)

	if err := db.AutoMigrate(&App{}, &Release{}); err != nil {
		t.Fatalf("legacy migrate: %v", err)
	}

	legacyApp := App{
		ID:            "app_repeat",
		Platform:      "android",
		ApplicationID: "com.example.repeat",
		Name:          "Repeat App",
		Published:     true,
	}
	if err := db.Create(&legacyApp).Error; err != nil {
		t.Fatalf("create legacy app: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate first pass: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate second pass: %v", err)
	}

	var productCount int64
	if err := db.Table("products").Where("legacy_app_id = ?", legacyApp.ID).Count(&productCount).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if productCount != 1 {
		t.Fatalf("expected 1 migrated product, got %d", productCount)
	}

	var variantCount int64
	if err := db.Table("variants").Where("legacy_app_id = ?", legacyApp.ID).Count(&variantCount).Error; err != nil {
		t.Fatalf("count variants: %v", err)
	}
	if variantCount != 1 {
		t.Fatalf("expected 1 migrated variant, got %d", variantCount)
	}
}

func TestAutoMigrateBackfillsLegacyBindingAndNonceVariantIDs(t *testing.T) {
	db := openTestDB(t)

	if err := db.AutoMigrate(&App{}, &Release{}, &DeviceAppBinding{}, &UDIDNonce{}); err != nil {
		t.Fatalf("legacy migrate: %v", err)
	}

	legacyApp := App{
		ID:        "app_binding",
		Platform:  "ios",
		BundleID:  "com.example.binding",
		Name:      "Binding App",
		Published: true,
	}
	if err := db.Create(&legacyApp).Error; err != nil {
		t.Fatalf("create legacy app: %v", err)
	}

	binding := DeviceAppBinding{
		ID:    "bind_legacy",
		AppID: legacyApp.ID,
		UDID:  "udid-1",
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("create legacy binding: %v", err)
	}

	nonce := UDIDNonce{
		Nonce: "nonce_legacy",
		AppID: legacyApp.ID,
	}
	if err := db.Create(&nonce).Error; err != nil {
		t.Fatalf("create legacy nonce: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	type variantRow struct {
		ID string
	}
	var variant variantRow
	if err := db.Table("variants").Where("legacy_app_id = ?", legacyApp.ID).Take(&variant).Error; err != nil {
		t.Fatalf("load variant: %v", err)
	}

	var migratedBinding DeviceAppBinding
	if err := db.Where("id = ?", binding.ID).Take(&migratedBinding).Error; err != nil {
		t.Fatalf("load binding: %v", err)
	}
	if migratedBinding.VariantID != variant.ID {
		t.Fatalf("expected binding variant_id %q, got %q", variant.ID, migratedBinding.VariantID)
	}

	var migratedNonce UDIDNonce
	if err := db.Where("nonce = ?", nonce.Nonce).Take(&migratedNonce).Error; err != nil {
		t.Fatalf("load nonce: %v", err)
	}
	if migratedNonce.VariantID != variant.ID {
		t.Fatalf("expected nonce variant_id %q, got %q", variant.ID, migratedNonce.VariantID)
	}
}

func TestProductSlugIsUnique(t *testing.T) {
	db := openTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	first := Product{ID: "prd_1", Slug: "demo", Name: "Demo"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first product: %v", err)
	}

	second := Product{ID: "prd_2", Slug: "demo", Name: "Demo 2"}
	if err := db.Create(&second).Error; err == nil {
		t.Fatalf("expected duplicate slug insert to fail")
	}
}
