package oss

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type OSS struct {
	cli       *minio.Client
	presignCli *minio.Client
}

func newMinioClient(address, accessKey, secretKey string) (*minio.Client, error) {
	endpoint := address
	secure := false

	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		u, err := url.Parse(address)
		if err != nil {
			return nil, err
		}
		if u.Path != "" && u.Path != "/" {
			return nil, errors.New("endpoint url cannot have fully qualified paths")
		}
		endpoint = u.Host
		secure = u.Scheme == "https"
	}

	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
		Region: "us-east-1",
	})
}

func NewOSSClient(address, publicAddress, accessKey, secretKey string) (*OSS, error) {
	cli, err := newMinioClient(address, accessKey, secretKey)
	if err != nil {
		return nil, err
	}

	presignCli := cli
	if strings.TrimSpace(publicAddress) != "" {
		presignCli, err = newMinioClient(publicAddress, accessKey, secretKey)
		if err != nil {
			return nil, err
		}
	}

	return &OSS{cli: cli, presignCli: presignCli}, nil
}

// CreateBucket 创建存储桶（如果不存在）
func (o *OSS) CreateBucket(ctx context.Context, bucket string) error {
	exists, err := o.cli.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		return o.cli.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}
	return nil
}

// UploadFile 服务器端直接上传文件流
func (o *OSS) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
	return o.cli.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

// DeleteFile 删除文件
func (o *OSS) DeleteFile(ctx context.Context, bucket, key string) error {
	return o.cli.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

// StatObject 获取对象元数据
func (o *OSS) StatObject(ctx context.Context, bucket, key string) (minio.ObjectInfo, error) {
	return o.cli.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
}

// GetObjectBytes 从 OSS 获取文件内容为字节数组
func (o *OSS) GetObjectBytes(ctx context.Context, bucket, key string) ([]byte, error) {
	obj, err := o.cli.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	b, err := io.ReadAll(obj)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// PresignGet 生成预签名下载链接
func (o *OSS) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := o.presignCli.PresignedGetObject(ctx, bucket, key, ttl, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// PresignPut 生成预签名上传链接
func (o *OSS) PresignPut(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	u, err := o.presignCli.PresignedPutObject(ctx, bucket, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// ListObjects 列出指定前缀下的对象
// recursive=true: 递归列出所有文件
// recursive=false: 模拟列出当前“目录”下的文件和子“目录”（以 / 结尾）
func (o *OSS) ListObjects(ctx context.Context, bucket, prefix string, recursive bool) ([]string, error) {
	var objects []string
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	}

	for object := range o.cli.ListObjects(ctx, bucket, opts) {
		if object.Err != nil {
			return nil, object.Err
		}
		// MinIO SDK 在非递归模式下，会自动把“文件夹”作为 Key 返回（以 / 结尾）
		objects = append(objects, object.Key)
	}
	return objects, nil
}

// ObjectInfo 对象元数据
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ContentType  string    `json:"content_type"`
	IsDir        bool      `json:"is_dir"`
}

// ListObjectsInfo 列出指定前缀下的对象详情
func (o *OSS) ListObjectsInfo(ctx context.Context, bucket, prefix string, recursive bool) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	}

	for object := range o.cli.ListObjects(ctx, bucket, opts) {
		if object.Err != nil {
			return nil, object.Err
		}
		objects = append(objects, ObjectInfo{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
			ContentType:  object.ContentType,
			IsDir:        strings.HasSuffix(object.Key, "/"),
		})
	}
	return objects, nil
}
