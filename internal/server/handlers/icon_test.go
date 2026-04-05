package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/openprx/fenfa/internal/store"
)

func TestProductIconServesStoredFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withUploadWorkdir(t)

	db := openHandlerTestDB(t)
	product := store.Product{
		ID:        "prd_icon",
		Slug:      "icon-suite",
		Name:      "Icon Suite",
		IconPath:  filepath.ToSlash(filepath.Join("uploads", "prd_icon", "icon.png")),
		Published: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(product.IconPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := []byte("png-icon-bytes")
	if err := os.WriteFile(product.IconPath, want, 0o644); err != nil {
		t.Fatalf("write icon: %v", err)
	}

	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/icon/products/"+product.ID, nil)
	c.Params = gin.Params{{Key: "productID", Value: product.ID}}

	ProductIcon(db)(c)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if body := rr.Body.Bytes(); string(body) != string(want) {
		t.Fatalf("expected icon body %q, got %q", want, body)
	}
}
