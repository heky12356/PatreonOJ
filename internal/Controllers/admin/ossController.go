package admin

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/oss"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OSSController struct {
	ossClient *oss.OSS
}

func NewOSSController(client *oss.OSS) *OSSController {
	return &OSSController{
		ossClient: client,
	}
}

// UploadFile 上传文件到 OSS
// POST /oss/upload
func (oc *OSSController) UploadFile(c *gin.Context) {
	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法获取上传文件"})
		return
	}
	defer file.Close()

	// 获取自定义路径前缀，默认为 "uploads/"
	prefix := c.PostForm("path")
	if prefix == "" {
		prefix = "uploads/"
	}
	// 确保前缀以 / 结尾
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	// 生成唯一文件名
	ext := filepath.Ext(header.Filename)
	filename := uuid.New().String() + ext
	key := prefix + filename

	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	// 上传到 OSS
	ctx := context.Background()
	info, err := oc.ossClient.UploadFile(ctx, bucket, key, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件上传失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "上传成功",
		"key":     key,
		"size":    info.Size,
		"etag":    info.ETag,
	})
}

// GetUploadURL 获取预签名上传链接（前端直传）
// GET /oss/upload-url?filename=xxx.txt&path=problems/1001/
func (oc *OSSController) GetUploadURL(c *gin.Context) {
	filename := c.Query("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename is required"})
		return
	}

	// 获取自定义路径前缀，默认为 "uploads/"
	prefix := c.Query("path")
	if prefix == "" {
		prefix = "uploads/"
	}
	// 确保前缀以 / 结尾
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	// 构造存储 Key
	key := prefix + filename

	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	ctx := context.Background()
	// 生成有效期为 10 分钟的预签名 PUT URL
	presignedURL, err := oc.ossClient.PresignPut(ctx, bucket, key, 10*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成上传链接失败: " + err.Error()})
		return
	}

	publicPrefix := strings.TrimSpace(config.GlobalConfig.OSS.PublicPathPrefix)
	if publicPrefix != "" {
		if !strings.HasPrefix(publicPrefix, "/") {
			publicPrefix = "/" + publicPrefix
		}
		if u, err := url.Parse(presignedURL); err == nil {
			u.Path = path.Join(publicPrefix, u.Path)
			presignedURL = u.String()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"url": presignedURL,
		"key": key,
	})
}

// ListFiles 列出 OSS 中的文件（模拟目录结构）
// GET /oss/files?prefix=problems/&recursive=false
func (oc *OSSController) ListFiles(c *gin.Context) {
	prefix := c.Query("prefix")
	recursive := c.Query("recursive") == "true"

	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	ctx := context.Background()
	// 使用 ListObjectsInfo 获取详细元数据
	objects, err := oc.ossClient.ListObjectsInfo(ctx, bucket, prefix, recursive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件列表失败: " + err.Error()})
		return
	}

	// 构造树状结构响应（如果需要更复杂的前端展示，可以在这里处理 objects）
	// 目前直接返回对象列表，包含 IsDir 字段供前端渲染目录树
	c.JSON(http.StatusOK, gin.H{
		"prefix":  prefix,
		"objects": objects,
	})
}
