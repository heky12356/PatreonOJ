package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/oss"

	"github.com/minio/minio-go/v7"
)

type mockOSSStore struct {
	uploadBucket string
	uploadKey    string

	presignBucket string
	presignKey    string
	presignURL    string
	presignErr    error

	listBucket string
	listPrefix string
	listRec    bool
	listResult []oss.ObjectInfo
	listErr    error
}

func (m *mockOSSStore) UploadFile(_ context.Context, bucket, key string, _ io.Reader, size int64, _ string) (minio.UploadInfo, error) {
	m.uploadBucket = bucket
	m.uploadKey = key
	return minio.UploadInfo{Size: size, ETag: "etag-1"}, nil
}

func (m *mockOSSStore) PresignPut(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	m.presignBucket = bucket
	m.presignKey = key
	if m.presignErr != nil {
		return "", m.presignErr
	}
	return m.presignURL, nil
}

func (m *mockOSSStore) ListObjectsInfo(_ context.Context, bucket, prefix string, recursive bool) ([]oss.ObjectInfo, error) {
	m.listBucket = bucket
	m.listPrefix = prefix
	m.listRec = recursive
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listResult, nil
}

func TestOSSService_UploadFile(t *testing.T) {
	store := &mockOSSStore{}
	svc := NewOSSService(store, "", "")

	out, err := svc.UploadFile(context.Background(), "problems/1001", "a.txt", bytes.NewBufferString("x"), 1, "text/plain")
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	if out.Bucket != "patreon-oj-cases" {
		t.Fatalf("unexpected bucket: %s", out.Bucket)
	}
	if store.uploadBucket != "patreon-oj-cases" {
		t.Fatalf("unexpected upload bucket: %s", store.uploadBucket)
	}
	if !strings.HasPrefix(store.uploadKey, "problems/1001/") {
		t.Fatalf("unexpected upload key: %s", store.uploadKey)
	}
}

func TestOSSService_BuildUploadURL(t *testing.T) {
	store := &mockOSSStore{presignURL: "http://minio.local/bucket/uploads/a.txt?x=1"}
	svc := NewOSSService(store, "bucket-a", "public")

	out, err := svc.BuildUploadURL(context.Background(), "a.txt", "uploads", 10*time.Minute)
	if err != nil {
		t.Fatalf("BuildUploadURL failed: %v", err)
	}
	if store.presignBucket != "bucket-a" || store.presignKey != "uploads/a.txt" {
		t.Fatalf("unexpected presign args: bucket=%s key=%s", store.presignBucket, store.presignKey)
	}
	if out.URL == store.presignURL {
		t.Fatalf("expected url rewritten with public prefix")
	}
}

func TestOSSService_BuildUploadURL_FilenameRequired(t *testing.T) {
	store := &mockOSSStore{}
	svc := NewOSSService(store, "bucket-a", "")

	_, err := svc.BuildUploadURL(context.Background(), " ", "uploads", 10*time.Minute)
	if !errors.Is(err, ErrOSSFilenameRequired) {
		t.Fatalf("expected ErrOSSFilenameRequired, got %v", err)
	}
}

func TestOSSService_ListFiles(t *testing.T) {
	store := &mockOSSStore{
		listResult: []oss.ObjectInfo{{Key: "a.txt"}},
	}
	svc := NewOSSService(store, "bucket-a", "")

	out, err := svc.ListFiles(context.Background(), "problems/", true)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if store.listBucket != "bucket-a" || store.listPrefix != "problems/" || !store.listRec {
		t.Fatalf("unexpected list args: bucket=%s prefix=%s recursive=%v", store.listBucket, store.listPrefix, store.listRec)
	}
	if len(out.Objects) != 1 || out.Objects[0].Key != "a.txt" {
		t.Fatalf("unexpected list result: %+v", out.Objects)
	}
}
