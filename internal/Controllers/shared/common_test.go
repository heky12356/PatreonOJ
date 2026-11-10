package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"patreonoj/internal/models"

	"github.com/gin-gonic/gin"
)

func TestRequireOperatorUUIDRejectsForgedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/identity", func(c *gin.Context) {
		if _, ok := RequireOperatorUUID(nil, c); ok {
			c.Status(http.StatusOK)
		}
	})
	req := httptest.NewRequest(http.MethodGet, "/identity?operator_uuid=admin", nil)
	req.Header.Set("X-User-UUID", "admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("forged identity accepted: %d", w.Code)
	}
}

func TestCanAccessUserStateRejectsDisabledSelf(t *testing.T) {
	if CanAccessUserState(models.User{Id: 1, Status: "disabled"}, "1") {
		t.Fatal("disabled account can still access its state")
	}
}
