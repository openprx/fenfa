package handlers

import (
	"encoding/json"
	"net/http"
	"time"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

type postEventReq struct {
	Type      string                 `json:"type"`
	AppID     string                 `json:"app_id"`
	VariantID string                 `json:"variant_id"`
	ReleaseID string                 `json:"release_id"`
	Extra     map[string]any         `json:"extra"`
}

// PostEvent accepts client-side events like visit/click and stores them into events table
func PostEvent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in postEventReq
		if err := c.ShouldBindJSON(&in); err != nil || in.Type == "" || (in.AppID == "" && in.VariantID == "" && in.ReleaseID == "") {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST"}})
			return
		}
		b, _ := json.Marshal(in.Extra)
		ev := store.Event{
			Type: in.Type,
			AppID: in.AppID,
			VariantID: in.VariantID,
			ReleaseID: in.ReleaseID,
			IP: c.ClientIP(),
			UA: c.Request.UserAgent(),
			Extra: string(b),
			Ts: time.Now(),
		}
		_ = db.Create(&ev).Error
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	}
}

// AdminListEvents lists events with filters; admin token required via middleware in router
func AdminListEvents(db *gorm.DB) gin.HandlerFunc {
		return func(c *gin.Context) {
			q := db.Model(&store.Event{})
			if v := c.Query("app_id"); v != "" { q = q.Where("app_id = ?", v) }
			if v := c.Query("variant_id"); v != "" { q = q.Where("variant_id = ?", v) }
			if v := c.Query("release_id"); v != "" { q = q.Where("release_id = ?", v) }
		if v := c.Query("type"); v != "" { q = q.Where("type = ?", v) }
		const layout = "2006-01-02"
		if from := c.Query("from"); from != "" { if t, err := time.Parse(layout, from); err == nil { q = q.Where("ts >= ?", t) } }
		if to := c.Query("to"); to != "" { if t, err := time.Parse(layout, to); err == nil { q = q.Where("ts < ?", t.Add(24*time.Hour)) } }
		var total int64
		q.Count(&total)
		limit := 100
		if v := c.Query("limit"); v != "" { if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 { limit = n } }
		offset := 0
		if v := c.Query("offset"); v != "" { if n, err := strconv.Atoi(v); err == nil && n >= 0 { offset = n } }
		var evs []store.Event
		if err := q.Order("ts desc").Limit(limit).Offset(offset).Find(&evs).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": []gin.H{}, "total": 0}})
			return
		}
		items := make([]gin.H, 0, len(evs))
			for _, e := range evs {
				items = append(items, gin.H{
					"ts": e.Ts, "type": e.Type, "app_id": e.AppID, "variant_id": e.VariantID, "release_id": e.ReleaseID,
					"ip": e.IP, "ua": e.UA, "extra": e.Extra,
				})
			}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"items": items, "total": total}})
	}
}
