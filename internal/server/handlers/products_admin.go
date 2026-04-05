package handlers

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

type adminVariantInput struct {
	Platform      string `json:"platform"`
	Identifier    string `json:"identifier"`
	DisplayName   string `json:"display_name"`
	Arch          string `json:"arch"`
	InstallerType string `json:"installer_type"`
	MinOS         string `json:"min_os"`
	Published     *bool  `json:"published"`
	SortOrder     *int   `json:"sort_order"`
}

type adminProductInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Published   *bool  `json:"published"`
}

func AdminListProducts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := db.Model(&store.Product{})
		if s := c.Query("q"); s != "" {
			like := "%" + s + "%"
			q = q.Where("name LIKE ? OR slug LIKE ?", like, like)
		}

		var total int64
		q.Count(&total)

		limit := 200
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		offset := 0
		if v := c.Query("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}

		var products []store.Product
		if err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&products).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": []gin.H{}, "total": 0}})
			return
		}

		items := make([]gin.H, 0, len(products))
		for _, product := range products {
			var variantCount int64
			db.Model(&store.Variant{}).Where("product_id = ?", product.ID).Count(&variantCount)
			items = append(items, gin.H{
				"id":            product.ID,
				"slug":          product.Slug,
				"name":          product.Name,
				"published":     product.Published,
				"variant_count": variantCount,
				"created_at":    product.CreatedAt,
				"updated_at":    product.UpdatedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": items, "total": total}})
	}
}

func AdminGetProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("productID")

		var product store.Product
		if err := db.Where("id = ?", productID).First(&product).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "product not found"}})
			return
		}

		var variants []store.Variant
		db.Where("product_id = ?", product.ID).Order("sort_order asc, created_at asc").Find(&variants)

		variantIDs := make([]string, 0, len(variants))
		for _, variant := range variants {
			variantIDs = append(variantIDs, variant.ID)
		}

		releaseMap := map[string][]store.Release{}
		if len(variantIDs) > 0 {
			var releases []store.Release
			db.Where("variant_id IN ?", variantIDs).Order("created_at desc").Find(&releases)
			for _, release := range releases {
				releaseMap[release.VariantID] = append(releaseMap[release.VariantID], release)
			}
		}

		profileMap := loadProvisioningProfilesByVariantReleases(db, releaseMap)
		items := make([]gin.H, 0, len(variants))
		for _, variant := range variants {
			releases := make([]gin.H, 0, len(releaseMap[variant.ID]))
			for _, release := range releaseMap[variant.ID] {
				item := gin.H{
					"id":             release.ID,
					"version":        release.Version,
					"build":          release.Build,
					"created_at":     release.CreatedAt,
					"download_count": release.DownloadCount,
					"channel":        release.Channel,
					"min_os":         release.MinOS,
					"changelog":      release.Changelog,
					"file_ext":       release.FileExt,
					"file_name":      release.FileName,
				}
				if profile, ok := profileMap[release.ID]; ok {
					item["provisioning_profile"] = formatProvisioningProfile(profile)
				}
				releases = append(releases, item)
			}
			items = append(items, gin.H{
				"id":             variant.ID,
				"platform":       variant.Platform,
				"identifier":     variant.Identifier,
				"legacy_app_id":  variant.LegacyAppID,
				"display_name":   variant.DisplayName,
				"arch":           variant.Arch,
				"installer_type": variant.InstallerType,
				"min_os":         variant.MinOS,
				"published":      variant.Published,
				"sort_order":     variant.SortOrder,
				"releases":       releases,
			})
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
			"product": gin.H{
				"id":          product.ID,
				"slug":        product.Slug,
				"name":        product.Name,
				"description": product.Description,
				"published":   product.Published,
				"created_at":  product.CreatedAt,
				"updated_at":  product.UpdatedAt,
			},
			"variants": items,
		}})
	}
}

func AdminGetVariantStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Param("variantID")

		var variant store.Variant
		if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "variant not found"}})
			return
		}

		var product store.Product
		if err := db.Where("id = ?", variant.ProductID).First(&product).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "product not found"}})
			return
		}

		var releases []store.Release
		db.Where("variant_id = ?", variant.ID).Order("created_at desc").Find(&releases)

		items := make([]gin.H, 0, len(releases))
		for _, release := range releases {
			items = append(items, gin.H{
				"id":             release.ID,
				"version":        release.Version,
				"build":          release.Build,
				"download_count": release.DownloadCount,
				"created_at":     release.CreatedAt,
				"channel":        release.Channel,
				"min_os":         release.MinOS,
				"changelog":      release.Changelog,
				"file_ext":       release.FileExt,
				"file_name":      release.FileName,
			})
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
			"product": gin.H{
				"id":   product.ID,
				"slug": product.Slug,
				"name": product.Name,
			},
			"variant": gin.H{
				"id":             variant.ID,
				"product_id":     variant.ProductID,
				"platform":       variant.Platform,
				"identifier":     variant.Identifier,
				"display_name":   variant.DisplayName,
				"arch":           variant.Arch,
				"installer_type": variant.InstallerType,
				"min_os":         variant.MinOS,
				"published":      variant.Published,
			},
			"releases": items,
		}})
	}
}

func AdminCreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input adminProductInput
		if err := c.ShouldBindJSON(&input); err != nil || input.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "invalid product payload"}})
			return
		}

		slug := adminSlugify(input.Slug)
		if slug == "" {
			slug = adminSlugify(input.Name)
		}
		if slug == "" {
			slug = "product"
		}

		published := true
		if input.Published != nil {
			published = *input.Published
		}

		product := store.Product{
			ID:          "prd_" + randHexN(8),
			Name:        input.Name,
			Slug:        nextAdminProductSlug(db, slug),
			Description: input.Description,
			Published:   published,
		}

		if err := db.Select("*").Create(&product).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}
		if product.Published != published {
			if err := db.Model(&product).Update("published", published).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
				return
			}
			product.Published = published
		}

		c.JSON(http.StatusCreated, gin.H{"ok": true, "data": gin.H{"product": gin.H{
			"id":          product.ID,
			"slug":        product.Slug,
			"name":        product.Name,
			"description": product.Description,
			"published":   product.Published,
			"created_at":  product.CreatedAt,
			"updated_at":  product.UpdatedAt,
		}}})
	}
}

func AdminUpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("productID")
		var input adminProductInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "invalid product payload"}})
			return
		}

		var product store.Product
		if err := db.Where("id = ?", productID).First(&product).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "product not found"}})
			return
		}

		updates := map[string]any{}
		if input.Name != "" {
			updates["name"] = input.Name
		}
		if input.Description != "" {
			updates["description"] = input.Description
		}
		if input.Published != nil {
			updates["published"] = *input.Published
		}
		if input.Slug != "" {
			slug := adminSlugify(input.Slug)
			if slug == "" {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "invalid slug"}})
				return
			}
			if slug != product.Slug {
				var existing store.Product
				if err := db.Where("slug = ? AND id <> ?", slug, product.ID).First(&existing).Error; err == nil {
					c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "slug already exists"}})
					return
				}
			}
			updates["slug"] = slug
		}

		if len(updates) > 0 {
			if err := db.Model(&product).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
				return
			}
		}

		if err := db.Where("id = ?", productID).First(&product).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"product": gin.H{
			"id":          product.ID,
			"slug":        product.Slug,
			"name":        product.Name,
			"description": product.Description,
			"published":   product.Published,
			"created_at":  product.CreatedAt,
			"updated_at":  product.UpdatedAt,
		}}})
	}
}

func AdminCreateVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("productID")
		var input adminVariantInput
		if err := c.ShouldBindJSON(&input); err != nil || input.Platform == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "invalid variant payload"}})
			return
		}

		var product store.Product
		if err := db.Where("id = ?", productID).First(&product).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "product not found"}})
			return
		}

		published := true
		if input.Published != nil {
			published = *input.Published
		}

		variant := store.Variant{
			ID:            "var_" + randHexN(8),
			ProductID:     product.ID,
			Platform:      input.Platform,
			Identifier:    input.Identifier,
			DisplayName:   input.DisplayName,
			Arch:          input.Arch,
			InstallerType: input.InstallerType,
			MinOS:         input.MinOS,
			Published:     published,
		}
		if input.SortOrder != nil {
			variant.SortOrder = *input.SortOrder
		}
		if variant.DisplayName == "" {
			variant.DisplayName = product.Name
		}

		if err := db.Select("*").Create(&variant).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}
		if variant.Published != published {
			if err := db.Model(&variant).Update("published", published).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
				return
			}
			variant.Published = published
		}

		c.JSON(http.StatusCreated, gin.H{"ok": true, "data": gin.H{"variant": gin.H{
			"id":             variant.ID,
			"product_id":     variant.ProductID,
			"platform":       variant.Platform,
			"identifier":     variant.Identifier,
			"legacy_app_id":  variant.LegacyAppID,
			"display_name":   variant.DisplayName,
			"arch":           variant.Arch,
			"installer_type": variant.InstallerType,
			"min_os":         variant.MinOS,
			"published":      variant.Published,
			"sort_order":     variant.SortOrder,
		}}})
	}
}

func AdminUpdateVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Param("variantID")
		var input adminVariantInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "invalid variant payload"}})
			return
		}

		var variant store.Variant
		if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "variant not found"}})
			return
		}

		updates := map[string]any{}
		if input.Platform != "" {
			updates["platform"] = input.Platform
		}
		if input.Identifier != "" {
			updates["identifier"] = input.Identifier
		}
		if input.DisplayName != "" {
			updates["display_name"] = input.DisplayName
		}
		if input.Arch != "" {
			updates["arch"] = input.Arch
		}
		if input.InstallerType != "" {
			updates["installer_type"] = input.InstallerType
		}
		if input.MinOS != "" {
			updates["min_os"] = input.MinOS
		}
		if input.Published != nil {
			updates["published"] = *input.Published
		}
		if input.SortOrder != nil {
			updates["sort_order"] = *input.SortOrder
		}
		if len(updates) > 0 {
			if err := db.Model(&variant).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
				return
			}
		}

		if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"variant": gin.H{
			"id":             variant.ID,
			"product_id":     variant.ProductID,
			"platform":       variant.Platform,
			"identifier":     variant.Identifier,
			"legacy_app_id":  variant.LegacyAppID,
			"display_name":   variant.DisplayName,
			"arch":           variant.Arch,
			"installer_type": variant.InstallerType,
			"min_os":         variant.MinOS,
			"published":      variant.Published,
			"sort_order":     variant.SortOrder,
		}}})
	}
}

func adminSlugify(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func nextAdminProductSlug(db *gorm.DB, base string) string {
	slug := base
	idx := 1
	for {
		var count int64
		db.Model(&store.Product{}).Where("slug = ?", slug).Count(&count)
		if count == 0 {
			return slug
		}
		idx++
		slug = base + "-" + strconv.Itoa(idx)
	}
}

// deleteReleaseFiles removes the file from disk for a release; errors are logged but not fatal.
func deleteReleaseFiles(releases []store.Release) {
	for _, r := range releases {
		if r.StoragePath == "" {
			continue
		}
		if err := os.Remove(r.StoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("WARN: failed to delete file %s: %v", r.StoragePath, err)
		}
	}
}

func AdminDeleteProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		productID := c.Param("productID")

		var product store.Product
		if err := db.Where("id = ?", productID).First(&product).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "product not found"}})
			return
		}

		var variants []store.Variant
		db.Where("product_id = ?", product.ID).Find(&variants)

		variantIDs := make([]string, 0, len(variants))
		for _, v := range variants {
			variantIDs = append(variantIDs, v.ID)
		}

		var releases []store.Release
		if len(variantIDs) > 0 {
			db.Where("variant_id IN ?", variantIDs).Find(&releases)
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if len(releases) > 0 {
				releaseIDs := make([]string, 0, len(releases))
				for _, r := range releases {
					releaseIDs = append(releaseIDs, r.ID)
				}
				if err := tx.Where("release_id IN ?", releaseIDs).Delete(&store.ProvisioningProfile{}).Error; err != nil {
					return err
				}
			}
			if len(variantIDs) > 0 {
				if err := tx.Where("variant_id IN ?", variantIDs).Delete(&store.DeviceAppBinding{}).Error; err != nil {
					return err
				}
				if err := tx.Where("variant_id IN ?", variantIDs).Delete(&store.Release{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("product_id = ?", product.ID).Delete(&store.Variant{}).Error; err != nil {
				return err
			}
			return tx.Delete(&product).Error
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}

		// Delete files from disk after successful transaction
		deleteReleaseFiles(releases)
		if product.IconPath != "" {
			if err := os.Remove(product.IconPath); err != nil && !os.IsNotExist(err) {
				log.Printf("WARN: failed to delete product icon %s: %v", product.IconPath, err)
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func AdminDeleteVariant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		variantID := c.Param("variantID")

		var variant store.Variant
		if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "variant not found"}})
			return
		}

		var releases []store.Release
		db.Where("variant_id = ?", variant.ID).Find(&releases)

		err := db.Transaction(func(tx *gorm.DB) error {
			if len(releases) > 0 {
				releaseIDs := make([]string, 0, len(releases))
				for _, r := range releases {
					releaseIDs = append(releaseIDs, r.ID)
				}
				if err := tx.Where("release_id IN ?", releaseIDs).Delete(&store.ProvisioningProfile{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("variant_id = ?", variant.ID).Delete(&store.DeviceAppBinding{}).Error; err != nil {
				return err
			}
			if err := tx.Where("variant_id = ?", variant.ID).Delete(&store.Release{}).Error; err != nil {
				return err
			}
			return tx.Delete(&variant).Error
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}

		// Delete files from disk after successful transaction
		deleteReleaseFiles(releases)

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func AdminDeleteRelease(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID := c.Param("releaseID")

		var release store.Release
		if err := db.Where("id = ?", releaseID).First(&release).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "release not found"}})
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("release_id = ?", release.ID).Delete(&store.ProvisioningProfile{}).Error; err != nil {
				return err
			}
			return tx.Delete(&release).Error
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "DB", "message": err.Error()}})
			return
		}

		// Delete file from disk after successful transaction
		deleteReleaseFiles([]store.Release{release})

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
