package site

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"patreonoj/internal/middleware"
	"patreonoj/internal/models"
	"patreonoj/internal/services/site"
)

func TestPublicReadAndAdminOnlyMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:site-controller?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.SiteSettings{}); err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	ctrl := &Controller{service: site.NewService(db)}
	for _, role := range []string{"anonymous", "user", "admin"} {
		t.Run(role, func(t *testing.T) {
			r := gin.New()
			if role != "anonymous" {
				r.Use(func(c *gin.Context) {
					p := ""
					if role == "admin" {
						p = "admin"
					}
					c.Set("currentUser", models.User{Id: 1, UUID: "example", Status: "active", Permissions: p})
					c.Next()
				})
			}
			r.GET("/settings", ctrl.Get)
			r.PUT("/settings", middleware.RequirePermission("admin"), ctrl.Update)
			r.POST("/images", middleware.RequirePermission("admin"), ctrl.UploadImage)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/settings", nil))
			if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("public settings: %d", w.Code)
			}
			for _, endpoint := range []string{"/settings", "/images"} {
				method := "PUT"
				if endpoint == "/images" {
					method = "POST"
				}
				req := httptest.NewRequest(method, endpoint, strings.NewReader(`{"theme_id":"default"}`))
				req.Header.Set("Content-Type", "application/json")
				w = httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if role != "admin" && w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
					t.Fatalf("%s mutation allowed: %d", role, w.Code)
				}
				if role == "admin" && endpoint == "/settings" && w.Code != 200 {
					t.Fatalf("admin write failed: %s", w.Body.String())
				}
				if role == "admin" && endpoint == "/images" && w.Code != 503 {
					t.Fatalf("expected unavailable storage, got %d", w.Code)
				}
			}
		})
	}
}

func TestImageEndpointRejectsOtherNamespaces(t *testing.T) {
	r := gin.New()
	c := &Controller{}
	r.GET("/images/:name", c.Image)
	for _, name := range []string{"answer.out", "problems%2F1%2Fsecret.out", "11111111-1111-4111-8111-111111111111.svg", "11111111-1111-4111-8111-111111111111.png"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/images/"+name, nil))
		if w.Code != 404 {
			t.Fatalf("unexpected public object access: %s -> %d", name, w.Code)
		}
	}
}
