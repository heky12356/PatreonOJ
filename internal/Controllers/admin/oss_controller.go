package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/oss"
	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
)

type OSSController struct {
	service *services.OSSService
}

func NewOSSController(client *oss.OSS) *OSSController {
	return &OSSController{
		service: services.NewOSSService(client, config.GlobalConfig.OSS.BucketName, config.GlobalConfig.OSS.PublicPathPrefix),
	}
}

// UploadFile POST /oss/upload
func (oc *OSSController) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		apiError(c, http.StatusBadRequest, "无法获取上传文件")
		return
	}
	defer file.Close()

	result, err := oc.service.UploadFile(
		context.Background(),
		c.PostForm("path"),
		header.Filename,
		file,
		header.Size,
		header.Header.Get("Content-Type"),
	)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "文件上传失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusOK, "上传成功", gin.H{
		"key":    result.Key,
		"size":   result.Size,
		"etag":   result.ETag,
		"bucket": result.Bucket,
	})
}

// GetUploadURL GET /oss/upload-url?filename=xxx.txt&path=problems/1001/
func (oc *OSSController) GetUploadURL(c *gin.Context) {
	result, err := oc.service.BuildUploadURL(
		context.Background(),
		c.Query("filename"),
		c.Query("path"),
		10*time.Minute,
	)
	if errors.Is(err, services.ErrOSSFilenameRequired) {
		apiError(c, http.StatusBadRequest, "filename is required")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "生成上传链接失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"url":    result.URL,
		"key":    result.Key,
		"bucket": result.Bucket,
	})
}

// ListFiles GET /oss/files?prefix=problems/&recursive=false
func (oc *OSSController) ListFiles(c *gin.Context) {
	recursive := strings.EqualFold(strings.TrimSpace(c.Query("recursive")), "true")
	result, err := oc.service.ListFiles(context.Background(), c.Query("prefix"), recursive)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "获取文件列表失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"prefix":  result.Prefix,
		"objects": result.Objects,
	})
}
