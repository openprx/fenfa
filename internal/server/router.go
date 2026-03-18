package server

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/openprx/fenfa/internal/config"
	"gorm.io/gorm"
)

// RegisterRoutes wires all HTTP routes into the Gin engine.
func RegisterRoutes(g *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Health check
	g.GET("/healthz", func(c *gin.Context) { ok(c, gin.H{"status": "ok"}) })

	// Public routes
	g.GET("/products/:slug", handleProductPage(db, cfg))
	g.GET("/d/:releaseID", handleDownload(db, cfg))
	g.GET("/manifest/:releaseID", handleManifest(db, cfg))

	// UDID enrollment
	g.GET("/udid/enroll/:nonce", handleUDIDEnroll(db, cfg))
	g.POST("/udid/callback", handleUDIDCallback(db, cfg))

	// Upload (requires upload or admin token)
	g.POST("/upload", authMiddleware(cfg, "upload"), handleUpload(db, cfg))

	// Admin API
	admin := g.Group("/admin/api", authMiddleware(cfg, "admin"))
	{
		// Upload config
		admin.GET("/upload-config", handleUploadConfig(cfg))

		// Parse app metadata
		admin.POST("/parse-app", handleParseApp())

		// Products
		admin.GET("/products", handleListProducts(db))
		admin.GET("/products/:id", handleGetProduct(db))
		admin.POST("/products", handleCreateProduct(db))
		admin.PUT("/products/:id", handleUpdateProduct(db))
		admin.DELETE("/products/:id", handleDeleteProduct(db))

		// Variants
		admin.POST("/products/:id/variants", handleCreateVariant(db))
		admin.PUT("/variants/:id", handleUpdateVariant(db))
		admin.DELETE("/variants/:id", handleDeleteVariant(db))

		// Settings
		admin.GET("/settings", handleGetSettings(db))
		admin.PUT("/settings", handleUpdateSettings(db))

		// Events & Stats
		admin.GET("/events", handleListEvents(db))
		admin.GET("/variants/:id/stats", handleVariantStats(db))

		// iOS devices
		admin.GET("/ios_variants", handleListIOSVariants(db))
		admin.GET("/ios_devices", handleListIOSDevices(db))

		// Apple device registration
		admin.GET("/apple/status", handleAppleStatus(db))
		admin.POST("/devices/:id/register-apple", handleRegisterApple(db))
		admin.POST("/devices/register-apple", handleBatchRegisterApple(db))
	}

	// Admin exports
	exports := g.Group("/admin/exports", authMiddleware(cfg, "admin"))
	{
		exports.GET("/releases.csv", handleExportReleases(db))
		exports.GET("/events.csv", handleExportEvents(db))
		exports.GET("/ios_devices.csv", handleExportIOSDevices(db))
	}
}

func newID(prefix string) string {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "00000"
	}
	return prefix + hex.EncodeToString(buf)[:8]
}
