package site

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/config"
	"patreonoj/internal/oss"
	sitesvc "patreonoj/internal/services/site"
)

const maxImageSize = 8 << 20

type Controller struct {
	service *sitesvc.Service
	objects *oss.OSS
	bucket  string
}

func NewController(db *gorm.DB, objects *oss.OSS) *Controller {
	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}
	return &Controller{service: sitesvc.NewService(db), objects: objects, bucket: bucket}
}

func (c *Controller) Get(ctx *gin.Context) {
	settings, err := c.service.Get()
	if err != nil {
		shared.APIError(ctx, 500, "读取站点设置失败")
		return
	}
	ctx.Header("Cache-Control", "no-store")
	shared.APIOK(ctx, 200, "ok", gin.H{"data": settings})
}

func (c *Controller) Update(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 64<<10)
	var input sitesvc.Update
	if err := ctx.ShouldBindJSON(&input); err != nil {
		shared.APIError(ctx, 400, "设置格式无效")
		return
	}
	settings, err := c.service.Update(input)
	if errors.Is(err, sitesvc.ErrInvalidSettings) {
		shared.APIError(ctx, 400, "设置无效：检查主题、图片与链接；最多 10 张图片，轮播间隔为 3 到 30 秒")
		return
	}
	if err != nil {
		shared.APIError(ctx, 500, "保存站点设置失败")
		return
	}
	ctx.Header("Cache-Control", "no-store")
	shared.APIOK(ctx, 200, "设置已保存", gin.H{"data": settings})
}

// Public images use a dedicated namespace; test data never passes through it.
func (c *Controller) UploadImage(ctx *gin.Context) {
	if c.objects == nil {
		shared.APIError(ctx, 503, "图片存储暂不可用")
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxImageSize+(1<<20))
	file, _, err := ctx.Request.FormFile("file")
	if ctx.Request.MultipartForm != nil {
		defer ctx.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		shared.APIError(ctx, 400, "请选择不超过 8 MB 的 JPG、PNG 或 GIF 图片")
		return
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil || len(body) > maxImageSize {
		shared.APIError(ctx, 413, "图片不能超过 8 MB")
		return
	}
	size, format, err := image.DecodeConfig(bytes.NewReader(body))
	ext := map[string]string{"jpeg": "jpg", "png": "png", "gif": "gif"}[format]
	if err != nil || ext == "" || size.Width < 1 || size.Height < 1 || size.Width > 12000 || size.Height > 12000 || int64(size.Width)*int64(size.Height) > 40000000 {
		shared.APIError(ctx, 400, "图片格式或尺寸无效，请使用 JPG、PNG 或 GIF")
		return
	}
	name := uuid.NewString() + "." + ext
	_, err = c.objects.UploadFile(ctx.Request.Context(), c.bucket, "site/slides/"+name, bytes.NewReader(body), int64(len(body)), http.DetectContentType(body))
	if err != nil {
		shared.APIError(ctx, 500, "图片上传失败")
		return
	}
	shared.APIOK(ctx, 201, "图片已上传", gin.H{"data": gin.H{"image_url": "/api/site/images/" + name}})
}

func (c *Controller) Image(ctx *gin.Context) {
	name := ctx.Param("name")
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		ctx.Status(404)
		return
	}
	id, err := uuid.Parse(parts[0])
	contentType := map[string]string{"jpg": "image/jpeg", "png": "image/png", "gif": "image/gif"}[parts[1]]
	if err != nil || id.String() != parts[0] || contentType == "" || c.objects == nil {
		ctx.Status(404)
		return
	}
	body, err := c.objects.GetObjectBytes(ctx.Request.Context(), c.bucket, "site/slides/"+name)
	if err != nil {
		ctx.Status(404)
		return
	}
	etag := fmt.Sprintf("\"%x\"", sha256.Sum256(body))
	ctx.Header("Cache-Control", "public, max-age=86400, immutable")
	ctx.Header("ETag", etag)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	if ctx.GetHeader("If-None-Match") == etag {
		ctx.Status(http.StatusNotModified)
		return
	}
	ctx.Data(200, contentType, body)
}
