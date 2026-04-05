package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/store"
)

func openEventsTestDB(t *testing.T) *gorm.DB {
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

func TestPostEventAcceptsReleaseOnlyPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openEventsTestDB(t)

	body, err := json.Marshal(map[string]any{
		"type":       "visit",
		"variant_id": "var_windows",
		"release_id": "rel_windows",
		"extra": map[string]any{
			"path": "/products/demo-suite",
		},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	PostEvent(db)(c)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var event store.Event
	if err := db.Where("release_id = ?", "rel_windows").First(&event).Error; err != nil {
		t.Fatalf("expected event stored: %v", err)
	}
	if event.ReleaseID != "rel_windows" {
		t.Fatalf("expected release_id rel_windows, got %q", event.ReleaseID)
	}
	if event.VariantID != "var_windows" {
		t.Fatalf("expected variant_id var_windows, got %q", event.VariantID)
	}
	if event.AppID != "" {
		t.Fatalf("expected empty app_id for release-only event, got %q", event.AppID)
	}
}

func TestAdminListEventsFiltersByVariantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openEventsTestDB(t)

	events := []store.Event{
		{Type: "click", VariantID: "var_a", ReleaseID: "rel_a"},
		{Type: "click", VariantID: "var_b", ReleaseID: "rel_b"},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("create events: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/api/events?variant_id=var_b", nil)

	AdminListEvents(db)(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items := payload["data"].(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 event, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["variant_id"] != "var_b" {
		t.Fatalf("expected variant_id var_b, got %v", item["variant_id"])
	}
}
