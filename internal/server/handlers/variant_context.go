package handlers

import (
	"errors"

	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

type releaseVariantContext struct {
	Variant store.Variant
	Product store.Product
}

func loadReleaseVariantContext(db *gorm.DB, rel store.Release) (releaseVariantContext, error) {
	if rel.VariantID == "" {
		return releaseVariantContext{}, gorm.ErrRecordNotFound
	}

	var variant store.Variant
	if err := db.Where("id = ?", rel.VariantID).First(&variant).Error; err != nil {
		return releaseVariantContext{}, err
	}

	var product store.Product
	if err := db.Where("id = ?", variant.ProductID).First(&product).Error; err != nil {
		return releaseVariantContext{}, err
	}

	return releaseVariantContext{
		Variant: variant,
		Product: product,
	}, nil
}

func loadVariantProductContext(db *gorm.DB, variantID string) (releaseVariantContext, error) {
	var variant store.Variant
	if err := db.Where("id = ?", variantID).First(&variant).Error; err != nil {
		return releaseVariantContext{}, err
	}

	var product store.Product
	if err := db.Where("id = ?", variant.ProductID).First(&product).Error; err != nil {
		return releaseVariantContext{}, err
	}

	return releaseVariantContext{
		Variant: variant,
		Product: product,
	}, nil
}

func isMissingRecord(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
