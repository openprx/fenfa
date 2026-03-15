package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&Product{},
		&Variant{},
		&App{},
		&Release{},
		&AuthToken{},
		&Event{},
		&IOSDevice{},
		&DeviceAppBinding{},
		&SystemSettings{},
		&UDIDNonce{},
		&ProvisioningProfile{},
	)
	if err != nil {
		return err
	}

	if err := migrateLegacyApps(db); err != nil {
		return err
	}

	// Drop deprecated column if it exists (cleanup after removing bundle_id_prefix)
	if db.Migrator().HasColumn(&SystemSettings{}, "bundle_id_prefix") {
		_ = db.Migrator().DropColumn(&SystemSettings{}, "bundle_id_prefix")
	}

	// Initialize default system settings if not exists
	var settings SystemSettings
	if err := db.Where("id = ?", "default").First(&settings).Error; err == gorm.ErrRecordNotFound {
		settings = SystemSettings{
			ID:               "default",
			PrimaryDomain:    "http://localhost:8000",
			SecondaryDomains: "[]", // Empty JSON array
			Organization:     "Fenfa Distribution",
			UpdatedAt:        time.Now(),
		}
		if err := db.Create(&settings).Error; err != nil {
			return err
		}
	}

	return nil
}

func migrateLegacyApps(db *gorm.DB) error {
	var apps []App
	if err := db.Find(&apps).Error; err != nil {
		return err
	}

	for _, app := range apps {
		product, err := ensureLegacyProduct(db, app)
		if err != nil {
			return err
		}
		variant, err := ensureLegacyVariant(db, product, app)
		if err != nil {
			return err
		}
		if err := backfillLegacyReleaseVariant(db, app.ID, variant.ID); err != nil {
			return err
		}
		if err := backfillLegacyBindings(db, app.ID, variant.ID); err != nil {
			return err
		}
		if err := backfillLegacyNonces(db, app.ID, variant.ID); err != nil {
			return err
		}
	}

	return nil
}

func ensureLegacyProduct(db *gorm.DB, app App) (Product, error) {
	legacyID := app.ID
	productName := legacyProductName(app)

	var product Product
	err := db.Where("legacy_app_id = ?", legacyID).First(&product).Error
	if err == nil {
		updates := map[string]interface{}{
			"name":      productName,
			"icon_path": app.IconPath,
			"published": app.Published,
		}
		if product.Slug == "" {
			updates["slug"] = nextProductSlug(db, app.Name, app.ID)
		}
		if updateErr := db.Model(&product).Updates(updates).Error; updateErr != nil {
			return Product{}, updateErr
		}
		if reloadErr := db.Where("id = ?", product.ID).First(&product).Error; reloadErr != nil {
			return Product{}, reloadErr
		}
		return product, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return Product{}, err
	}

	product = Product{
		ID:          "prd_" + randHexN(8),
		Slug:        nextProductSlug(db, app.Name, app.ID),
		Name:        productName,
		IconPath:    app.IconPath,
		Published:   app.Published,
		LegacyAppID: &legacyID,
	}

	return product, db.Create(&product).Error
}

func ensureLegacyVariant(db *gorm.DB, product Product, app App) (Variant, error) {
	legacyID := app.ID
	displayName := legacyVariantDisplayName(app, product)

	var variant Variant
	err := db.Where("legacy_app_id = ?", legacyID).First(&variant).Error
	if err == nil {
		updates := map[string]interface{}{
			"product_id":     product.ID,
			"platform":       app.Platform,
			"identifier":     legacyIdentifier(app),
			"display_name":   displayName,
			"published":      app.Published,
			"installer_type": installerTypeForPlatform(app.Platform),
		}
		if updateErr := db.Model(&variant).Updates(updates).Error; updateErr != nil {
			return Variant{}, updateErr
		}
		if reloadErr := db.Where("id = ?", variant.ID).First(&variant).Error; reloadErr != nil {
			return Variant{}, reloadErr
		}
		return variant, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return Variant{}, err
	}

	variant = Variant{
		ID:            "var_" + randHexN(8),
		ProductID:     product.ID,
		Platform:      app.Platform,
		Identifier:    legacyIdentifier(app),
		DisplayName:   displayName,
		InstallerType: installerTypeForPlatform(app.Platform),
		Published:     app.Published,
		LegacyAppID:   &legacyID,
	}

	return variant, db.Create(&variant).Error
}

func backfillLegacyReleaseVariant(db *gorm.DB, appID, variantID string) error {
	return db.Model(&Release{}).
		Where("app_id = ? AND (variant_id = '' OR variant_id IS NULL)", appID).
		Update("variant_id", variantID).Error
}

func backfillLegacyBindings(db *gorm.DB, appID, variantID string) error {
	return db.Model(&DeviceAppBinding{}).
		Where("app_id = ? AND (variant_id = '' OR variant_id IS NULL)", appID).
		Update("variant_id", variantID).Error
}

func backfillLegacyNonces(db *gorm.DB, appID, variantID string) error {
	return db.Model(&UDIDNonce{}).
		Where("app_id = ? AND (variant_id = '' OR variant_id IS NULL)", appID).
		Update("variant_id", variantID).Error
}

func legacyIdentifier(app App) string {
	if app.BundleID != "" {
		return app.BundleID
	}
	if app.ApplicationID != "" {
		return app.ApplicationID
	}
	return app.ID
}

func legacyProductName(app App) string {
	if app.Name != "" {
		return app.Name
	}
	if app.BundleID != "" {
		return app.BundleID
	}
	if app.ApplicationID != "" {
		return app.ApplicationID
	}
	return app.ID
}

func legacyVariantDisplayName(app App, product Product) string {
	if app.Name != "" {
		return app.Name
	}
	if product.Name != "" {
		return product.Name
	}
	return legacyIdentifier(app)
}

func installerTypeForPlatform(platform string) string {
	switch strings.ToLower(platform) {
	case "ios":
		return "ipa"
	case "android":
		return "apk"
	case "macos":
		return "dmg"
	case "windows":
		return "exe"
	case "linux":
		return "appimage"
	default:
		return ""
	}
}

func nextProductSlug(db *gorm.DB, name, fallback string) string {
	base := slugify(name)
	if base == "" {
		base = slugify(fallback)
	}
	if base == "" {
		base = "product"
	}

	slug := base
	for i := 2; ; i++ {
		var count int64
		db.Model(&Product{}).Where("slug = ?", slug).Count(&count)
		if count == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func slugify(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func randHexN(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, (n+1)/2)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(buf)[:n]
}
