package shared

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestCtx(rawURL string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", rawURL, nil)
	c.Request = req
	return c
}

func TestParsePositiveIntQuery(t *testing.T) {
	c := newTestCtx("/?limit=12")
	v, err := ParsePositiveIntQuery(c, "limit", 10)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v != 12 {
		t.Fatalf("expected 12, got %d", v)
	}
}

func TestParsePositiveIntQuery_Default(t *testing.T) {
	c := newTestCtx("/")
	v, err := ParsePositiveIntQuery(c, "limit", 10)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v != 10 {
		t.Fatalf("expected default 10, got %d", v)
	}
}

func TestParsePositiveIntQuery_Invalid(t *testing.T) {
	c := newTestCtx("/?limit=abc")
	_, err := ParsePositiveIntQuery(c, "limit", 10)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParsePageSizeQuery_WithCap(t *testing.T) {
	c := newTestCtx("/?page=2&size=500")
	page, size, err := ParsePageSizeQuery(c, "page", "size", 1, 20, 100)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if page != 2 || size != 100 {
		t.Fatalf("expected page=2 size=100, got page=%d size=%d", page, size)
	}
}
