package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AppDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "app.html", gin.H{
			"Title":   "Modern distribution",
			"Message": "",
		})
	}
}

func LegacyAppRedirect(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("appID")
		if appID == "" {
			c.Status(http.StatusNotFound)
			return
		}
		ctx, err := loadVariantContextByLegacyAppID(db, appID)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		target := "/products/" + productPathKey(ctx.Product)
		if raw := c.Request.URL.RawQuery; raw != "" {
			target += "?" + raw
		}
		c.Redirect(http.StatusMovedPermanently, target)
	}
}
