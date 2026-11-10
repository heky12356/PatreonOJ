package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	storagesvc "patreonoj/internal/services/storage"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

type controllerStore struct {
	key, prefix string
	calls       int
}

func (s *controllerStore) UploadFile(_ context.Context, _, key string, _ io.Reader, size int64, _ string) (minio.UploadInfo, error) {
	s.key = key
	s.calls++
	return minio.UploadInfo{Size: size}, nil
}
func (s *controllerStore) PresignPut(_ context.Context, _, key string, _ time.Duration) (string, error) {
	s.key = key
	s.calls++
	return "https://example.invalid/put", nil
}
func (s *controllerStore) ListObjectsInfo(_ context.Context, _, prefix string, _ bool) ([]oss.ObjectInfo, error) {
	s.prefix = prefix
	s.calls++
	return []oss.ObjectInfo{}, nil
}

func TestOSSControllerAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path, permission string
		want                   int
		key, prefix            string
	}{
		{"deny-case-overwrite", "/url?path=problems/1/&filename=answer.out", "", 403, "", ""},
		{"deny-case-list", "/files?prefix=problems/", "", 403, "", ""},
		{"deny-other-user", "/url?path=avatars/another/&filename=a.png", "", 403, "", ""},
		{"own-avatar", "/url?path=avatars/&filename=a.png", "", 200, "avatars/u1/a.png", ""},
		{"own-list-default", "/files", "", 200, "", "uploads/u1/"},
		{"admin-case", "/url?path=problems/1/&filename=answer.out", "admin", 200, "problems/1/answer.out", ""},
		{"deny-traversal", "/url?path=avatars/../problems/&filename=x", "", 400, "", ""},
		{"deny-filename-traversal", "/url?path=avatars/&filename=../answer.out", "", 400, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &controllerStore{}
			ctrl := &OSSController{service: storagesvc.NewOSSService(store, "bucket", "")}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("currentUser", models.User{UUID: "u1", Status: "active", Permissions: tc.permission})
				c.Next()
			})
			r.GET("/url", ctrl.GetUploadURL)
			r.GET("/files", ctrl.ListFiles)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
			if store.key != tc.key || store.prefix != tc.prefix {
				t.Fatalf("key=%q prefix=%q", store.key, store.prefix)
			}
			if tc.want != 200 && store.calls != 0 {
				t.Fatal("unauthorized request reached object store")
			}
		})
	}
}

func TestOSSControllerRejectsUnauthenticatedAccess(t *testing.T) {
	r := gin.New()
	ctrl := &OSSController{}
	r.GET("/url", ctrl.GetUploadURL)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/url?filename=x", nil))
	if w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestOSSControllerUploadBodyLimit(t *testing.T) {
	r := gin.New()
	ctrl := &OSSController{}
	r.POST("/upload", ctrl.UploadFile)
	// Streaming an overlong multipart field must be rejected while parsing,
	// before creating an object or buffering an unbounded upload.
	body := io.MultiReader(strings.NewReader("--audit\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.bin\"\r\n\r\n"), io.LimitReader(zeroReader{}, storagesvc.MaxUploadSize+(2<<20)))
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=audit")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
