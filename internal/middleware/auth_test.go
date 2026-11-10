package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Session{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestSessionValidation(t *testing.T) {
	for _, state := range []string{"active", "expired", "revoked", "no-expiry", "disabled", "deleted-user", "missing"} {
		t.Run(state, func(t *testing.T) {
			db := newMiddlewareTestDB(t)
			user := models.User{Username: "test-user", Password: "unused", UUID: "test-uuid", Status: "active"}
			if state == "disabled" {
				user.Status = "disabled"
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(time.Hour)
			session := models.Session{SessionId: "test-session", UserId: user.Id, ExpiresAt: &expiry}
			if state == "expired" {
				expiry = time.Now().Add(-time.Second)
			}
			if state == "no-expiry" {
				session.ExpiresAt = nil
			}
			if state == "revoked" {
				now := time.Now()
				session.RevokedAt = &now
			}
			if state != "missing" {
				if err := db.Create(&session).Error; err != nil {
					t.Fatal(err)
				}
			}
			if state == "deleted-user" {
				if err := db.Delete(&user).Error; err != nil {
					t.Fatal(err)
				}
			}
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(LoadCurrentUser(db))
			r.GET("/private", RequireAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			want := http.StatusUnauthorized
			if state == "active" {
				want = http.StatusOK
			} else if state == "disabled" {
				want = http.StatusForbidden
			}
			if w.Code != want {
				t.Fatalf("expected %d, got %d body=%s", want, w.Code, w.Body)
			}
		})
	}
}

func TestAuthIgnoresIdentityHeaders(t *testing.T) {
	db := newMiddlewareTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LoadCurrentUser(db))
	r.GET("/private", RequireAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/private?operator_uuid=administrator", nil)
	req.Header.Set("X-User-UUID", "administrator")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("forged identity header was accepted: %d", w.Code)
	}
}

func TestAuthDatabaseFailureReturns503(t *testing.T) {
	db := newMiddlewareTestDB(t)
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LoadCurrentUser(db))
	for _, permission := range []bool{false, true} {
		path := fmt.Sprintf("/private/%t", permission)
		auth := RequireAuth()
		if permission {
			auth = RequirePermission("admin")
		}
		r.GET(path, auth, func(c *gin.Context) { c.Status(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("database error was reported as %d", w.Code)
		}
	}
}
