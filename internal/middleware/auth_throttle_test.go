package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthThrottleSharesRateAcrossEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	throttle := AuthThrottle(1, 2)
	r.POST("/login", throttle, func(c *gin.Context) { c.Status(200) })
	r.POST("/register", throttle, func(c *gin.Context) { c.Status(200) })
	for i, path := range []string{"/login", "/register", "/login"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		want := 200
		if i == 2 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("request %d: got %d want %d", i, w.Code, want)
		}
	}
}

func TestAuthThrottleRejectsExcessConcurrentWorkAndReleasesSlot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	throttle := AuthThrottle(1, 100)
	r.GET("/slow", throttle, func(c *gin.Context) { close(entered); <-release; c.Status(200) })
	r.GET("/fast", throttle, func(c *gin.Context) { c.Status(200) })
	go func() {
		defer close(done)
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/slow", nil))
	}()
	<-entered
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/fast", nil))
	close(release)
	<-done
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("excess request returned %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/fast", nil))
	if w.Code != 200 {
		t.Fatalf("slot was not released: %d", w.Code)
	}
}
