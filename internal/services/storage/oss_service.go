package storage

import (
	"context"
	"errors"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"patreonoj/internal/oss"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

var (
	ErrOSSFilenameRequired = errors.New("filename is required")
	ErrOSSInvalidPath      = errors.New("invalid object path")
	ErrOSSFileTooLarge     = errors.New("file exceeds 32 MiB limit")
)

const MaxUploadSize int64 = 32 << 20

type ossObjectStore interface {
	UploadFile(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (minio.UploadInfo, error)
	PresignPut(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)
	ListObjectsInfo(ctx context.Context, bucket, prefix string, recursive bool) ([]oss.ObjectInfo, error)
}

type OSSUploadResult struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	ETag   string `json:"etag"`
	Bucket string `json:"bucket"`
}

type OSSUploadURLResult struct {
	URL    string `json:"url"`
	Key    string `json:"key"`
	Bucket string `json:"bucket"`
}

type OSSListFilesResult struct {
	Prefix  string           `json:"prefix"`
	Objects []oss.ObjectInfo `json:"objects"`
}

type OSSService struct {
	store            ossObjectStore
	bucket           string
	publicPathPrefix string
}

func NewOSSService(store ossObjectStore, bucket, publicPathPrefix string) *OSSService {
	b := strings.TrimSpace(bucket)
	if b == "" {
		b = "patreon-oj-cases"
	}
	return &OSSService{
		store:            store,
		bucket:           b,
		publicPathPrefix: strings.TrimSpace(publicPathPrefix),
	}
}

func (s *OSSService) UploadFile(ctx context.Context, prefix, originalFilename string, reader io.Reader, size int64, contentType string) (*OSSUploadResult, error) {
	if size < 0 || size > MaxUploadSize {
		return nil, ErrOSSFileTooLarge
	}
	prefix, err := NormalizePrefix(prefix)
	if err != nil {
		return nil, err
	}
	key := prefix + uuid.New().String() + filepath.Ext(originalFilename)
	info, err := s.store.UploadFile(ctx, s.bucket, key, reader, size, contentType)
	if err != nil {
		return nil, err
	}
	return &OSSUploadResult{
		Key:    key,
		Size:   info.Size,
		ETag:   info.ETag,
		Bucket: s.bucket,
	}, nil
}

func (s *OSSService) BuildUploadURL(ctx context.Context, filename, prefix string, ttl time.Duration) (*OSSUploadURLResult, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, ErrOSSFilenameRequired
	}
	if filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\\x00") {
		return nil, ErrOSSInvalidPath
	}
	prefix, err := NormalizePrefix(prefix)
	if err != nil {
		return nil, err
	}
	key := prefix + filename
	presignedURL, err := s.store.PresignPut(ctx, s.bucket, key, ttl)
	if err != nil {
		return nil, err
	}

	presignedURL = applyPublicPathPrefix(presignedURL, s.publicPathPrefix)
	return &OSSUploadURLResult{
		URL:    presignedURL,
		Key:    key,
		Bucket: s.bucket,
	}, nil
}

func (s *OSSService) ListFiles(ctx context.Context, prefix string, recursive bool) (*OSSListFilesResult, error) {
	if prefix != "" {
		var err error
		prefix, err = NormalizePrefix(prefix)
		if err != nil {
			return nil, err
		}
	}
	objects, err := s.store.ListObjectsInfo(ctx, s.bucket, prefix, recursive)
	if err != nil {
		return nil, err
	}
	return &OSSListFilesResult{
		Prefix:  prefix,
		Objects: objects,
	}, nil
}

func NormalizePrefix(prefix string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = "uploads/"
	}
	if strings.ContainsAny(prefix, "\\\x00") {
		return "", ErrOSSInvalidPath
	}
	for _, part := range strings.Split(strings.TrimSuffix(prefix, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return "", ErrOSSInvalidPath
		}
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix, nil
}

func applyPublicPathPrefix(rawURL, publicPathPrefix string) string {
	p := strings.TrimSpace(publicPathPrefix)
	if p == "" {
		return rawURL
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Path = path.Join(p, u.Path)
	return u.String()
}
