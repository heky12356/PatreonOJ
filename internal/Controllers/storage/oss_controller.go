package storage

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/config"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	storagesvc "patreonoj/internal/services/storage"
	"patreonoj/internal/util"

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
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, storagesvc.MaxUploadSize+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var limitErr *http.MaxBytesError
		if errors.As(err, &limitErr) {
			shared.APIError(c, http.StatusRequestEntityTooLarge, "file exceeds 32 MiB limit")
			return
		}
		shared.APIError(c, http.StatusBadRequest, "failed to get upload file")
		return
	}
	defer file.Close()
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	prefix, ok := authorizedOSSPrefix(c, c.PostForm("path"), false)
	if !ok {
		return
	}

	result, err := oc.service.UploadFile(
		c.Request.Context(),
		prefix,
		header.Filename,
		file,
		header.Size,
		header.Header.Get("Content-Type"),
	)
	if err != nil {
		if errors.Is(err, storagesvc.ErrOSSFileTooLarge) {
			shared.APIError(c, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
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
	prefix, ok := authorizedOSSPrefix(c, c.Query("path"), false)
	if !ok {
		return
	}
	result, err := oc.service.BuildUploadURL(
		c.Request.Context(),
		c.Query("filename"),
		prefix,
		10*time.Minute,
	)
	if errors.Is(err, storagesvc.ErrOSSFilenameRequired) || errors.Is(err, storagesvc.ErrOSSInvalidPath) {
		shared.APIError(c, http.StatusBadRequest, err.Error())
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
	prefix, ok := authorizedOSSPrefix(c, c.Query("prefix"), true)
	if !ok {
		return
	}
	recursive := strings.EqualFold(strings.TrimSpace(c.Query("recursive")), "true")
	result, err := oc.service.ListFiles(c.Request.Context(), prefix, recursive)
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "list files failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{
		"prefix":  result.Prefix,
		"objects": result.Objects,
	})
}

// Ordinary accounts can only use their own upload namespaces. Administrators
// retain access to problem data, which is never writable through a user prefix.
func authorizedOSSPrefix(c *gin.Context, raw string, listing bool) (string, bool) {
	value, ok := c.Get("currentUser")
	user, valid := value.(models.User)
	if !ok || !valid {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	if util.UserInstance.HasPermission(user, "admin") && listing && strings.TrimSpace(raw) == "" {
		return "", true
	}
	prefix, err := storagesvc.NormalizePrefix(raw)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return "", false
	}
	if util.UserInstance.HasPermission(user, "admin") {
		return prefix, true
	}
	if user.UUID == "" || strings.ContainsAny(user.UUID, "/\\\x00") {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return "", false
	}
	for _, base := range []string{"avatars/", "uploads/"} {
		owned := base + user.UUID + "/"
		if prefix == base {
			return owned, true
		}
		if strings.HasPrefix(prefix, owned) {
			return prefix, true
		}
	}
	shared.APIError(c, http.StatusForbidden, "permission denied")
	return "", false
}
