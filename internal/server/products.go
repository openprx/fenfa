package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/store"
	"gorm.io/gorm"
)

func handleListProducts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := strings.TrimSpace(c.Query("q"))

		var products []store.Product
		tx := db.Order("created_at DESC")
		if q != "" {
			tx = tx.Where("name LIKE ? OR slug LIKE ?", "%"+q+"%", "%"+q+"%")
		}
		if err := tx.Find(&products).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		type item struct {
			store.Product
			VariantCount int64 `json:"variant_count"`
		}
		items := make([]item, 0, len(products))
		for _, p := range products {
			var count int64
			db.Model(&store.Variant{}).Where("product_id = ?", p.ID).Count(&count)
			items = append(items, item{Product: p, VariantCount: count})
		}

		ok(c, gin.H{"items": items})
	}
}

func handleGetProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var product store.Product
		if err := db.Where("id = ?", id).First(&product).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "product not found")
			return
		}

		var variants []store.Variant
		db.Where("product_id = ?", product.ID).Order("sort_order ASC, created_at ASC").Find(&variants)

		type variantWithReleases struct {
			store.Variant
			Releases []store.Release `json:"releases"`
		}
		vwr := make([]variantWithReleases, 0, len(variants))
		for _, v := range variants {
			var releases []store.Release
			db.Where("variant_id = ?", v.ID).Order("build DESC, created_at DESC").Find(&releases)
			vwr = append(vwr, variantWithReleases{Variant: v, Releases: releases})
		}

		ok(c, gin.H{"product": product, "variants": vwr})
	}
}

func handleCreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name        string `json:"name" binding:"required"`
			Slug        string `json:"slug"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		slug := req.Slug
		if slug == "" {
			slug = slugify(req.Name)
		}
		if slug == "" {
			slug = "product"
		}
		slug = ensureUniqueSlug(db, slug)

		product := store.Product{
			ID:          newID("prd_"),
			Name:        req.Name,
			Slug:        slug,
			Description: req.Description,
			Published:   true,
		}
		if err := db.Create(&product).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		ok(c, gin.H{"product": product, "variants": []store.Variant{}})
	}
}

func handleUpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var product store.Product
		if err := db.Where("id = ?", id).First(&product).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "product not found")
			return
		}

		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		if err := db.Model(&product).Updates(req).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		db.Where("id = ?", id).First(&product)

		ok(c, gin.H{"product": product})
	}
}

func handleDeleteProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var product store.Product
		if err := db.Where("id = ?", id).First(&product).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "product not found")
			return
		}

		// Delete associated variants and releases
		var variants []store.Variant
		db.Where("product_id = ?", id).Find(&variants)
		for _, v := range variants {
			db.Where("variant_id = ?", v.ID).Delete(&store.Release{})
		}
		db.Where("product_id = ?", id).Delete(&store.Variant{})
		db.Delete(&product)

		ok(c, gin.H{"deleted": true})
	}
}

func handleCreateVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("id")

		var product store.Product
		if err := db.Where("id = ?", productID).First(&product).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "product not found")
			return
		}

		var req struct {
			Platform    string `json:"platform" binding:"required"`
			DisplayName string `json:"display_name"`
			Identifier  string `json:"identifier"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		displayName := req.DisplayName
		if displayName == "" {
			displayName = product.Name + " (" + req.Platform + ")"
		}

		variant := store.Variant{
			ID:            newID("var_"),
			ProductID:     productID,
			Platform:      strings.ToLower(req.Platform),
			Identifier:    req.Identifier,
			DisplayName:   displayName,
			InstallerType: installerTypeForPlatform(req.Platform),
			Published:     true,
		}
		if err := db.Create(&variant).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		// Return full product detail
		handleGetProduct(db)(c)
	}
}

func handleUpdateVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var variant store.Variant
		if err := db.Where("id = ?", id).First(&variant).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "variant not found")
			return
		}

		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		if err := db.Model(&variant).Updates(req).Error; err != nil {
			fail(c, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		ok(c, gin.H{"variant": variant})
	}
}

func handleDeleteVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var variant store.Variant
		if err := db.Where("id = ?", id).First(&variant).Error; err != nil {
			fail(c, http.StatusNotFound, "not_found", "variant not found")
			return
		}

		db.Where("variant_id = ?", id).Delete(&store.Release{})
		db.Delete(&variant)

		ok(c, gin.H{"deleted": true})
	}
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
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func ensureUniqueSlug(db *gorm.DB, base string) string {
	slug := base
	for i := 2; ; i++ {
		var count int64
		db.Model(&store.Product{}).Where("slug = ?", slug).Count(&count)
		if count == 0 {
			return slug
		}
		slug = base + "-" + strings.Repeat("0", 0) + string(rune('0'+i%10))
		if i > 10 {
			slug = base + "-" + newID("")
		}
	}
}
