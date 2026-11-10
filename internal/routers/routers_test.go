package routers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/models"
	authsvc "patreonoj/internal/services/auth"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUserRoutesAndSessionLogout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	oldDB, oldConfig := models.DB, config.GlobalConfig
	models.DB = db
	config.GlobalConfig = &config.Config{Judge: config.JudgeConfig{QueueSize: 100}, Server: config.ServerConfig{SecureCookies: true, AuthMaxConcurrent: 2, AuthRequestsPerMinute: 100}}
	defer func() { models.DB, config.GlobalConfig = oldDB, oldConfig }()
	if err := models.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	hash, err := authsvc.HashPassword("test-password")
	if err != nil {
		t.Fatal(err)
	}
	owner := models.User{UUID: "owner-uuid", Username: "owner", Password: hash, Email: "private@example.test"}
	other := models.User{UUID: "other-uuid", Username: "other", Password: hash}
	admin := models.User{UUID: "admin-uuid", Username: "admin", Password: hash, Permissions: "admin"}
	for _, user := range []*models.User{&owner, &other, &admin} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	expiry := time.Now().Add(time.Hour)
	if err := db.Create(&models.Session{SessionId: "admin-session", UserId: admin.Id, ExpiresAt: &expiry}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	stop := RoutersInit(r, nil)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := stop(ctx); err != nil {
			t.Error(err)
		}
	}()
	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	w := request("POST", "/user/login", `{"username":"owner","password":"test-password"}`, nil)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatal("missing secure session cookie")
	}
	cookie := cookies[0]
	for _, tc := range []struct {
		method, path, body string
		cookie             *http.Cookie
		want               int
	}{
		{"GET", "/healthz", "", nil, 200},
		{"GET", "/user/owner-uuid", "", cookie, 200},
		{"GET", "/user/other-uuid", "", cookie, 403},
		{"PUT", "/user/owner-uuid", `{"nickname":"updated"}`, cookie, 200},
		{"PUT", "/user/other-uuid", `{"nickname":"hijacked"}`, cookie, 403},
		{"PUT", "/user/owner-uuid", `{"permissions":"admin"}`, cookie, 403},
		{"GET", "/user/owner-uuid", "", &http.Cookie{Name: "session_id", Value: "admin-session"}, 200},
	} {
		w := request(tc.method, tc.path, tc.body, tc.cookie)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
	w = request("GET", "/user/1/public", "", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "email") {
		t.Fatalf("public profile leaks email: %s", w.Body)
	}
	w = request("POST", "/user/logout", "", cookie)
	if w.Code != 200 || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout did not expire cookie: %s", w.Body)
	}
	w = request("GET", "/user/me", "", cookie)
	if w.Code != 401 {
		t.Fatalf("revoked cookie still works: %d", w.Code)
	}
}
