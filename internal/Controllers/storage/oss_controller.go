package storage

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/controllers/shared"
	"patreonoj/internal/oss"
	storagesvc "patreonoj/internal/services/storage"

	"github.com/gin-gonic/gin"
)

type OSSController struct {
	service *storagesvc.OSSService
}

func NewOSSController(client *oss.OSS) *OSSController {
	return &OSSController{
		service: storagesvc.NewOSSService(client, config.GlobalConfig.OSS.BucketName, config.GlobalConfig.OSS.PublicPathPrefix),
	}
}

// UploadFile POST /oss/upload
func (oc *OSSController) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, "failed to get upload file")
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
		shared.APIError(c, http.StatusInternalServerError, "upload failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "upload success", gin.H{
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
	if errors.Is(err, storagesvc.ErrOSSFilenameRequired) {
		shared.APIError(c, http.StatusBadRequest, "filename is required")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "build upload url failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{
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
		shared.APIError(c, http.StatusInternalServerError, "list files failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{
		"prefix":  result.Prefix,
		"objects": result.Objects,
	})
}
