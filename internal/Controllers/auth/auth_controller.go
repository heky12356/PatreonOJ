package auth

import (
	"errors"
	"net/http"
	"patreonoj/internal/config"
	"time"

	"patreonoj/internal/Controllers/shared"
	authsvc "patreonoj/internal/services/auth"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthController struct {
	db          *gorm.DB
	authService *authsvc.AuthService
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{
		db:          db,
		authService: authsvc.NewAuthService(db),
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (ac *AuthController) Login(c *gin.Context) {
	var loginReq LoginRequest
	if !bindAuthJSON(c, &loginReq) {
		return
	}

	result, err := ac.authService.Login(loginReq.Username, loginReq.Password)
	if errors.Is(err, authsvc.ErrAuthInvalidInput) {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, authsvc.ErrAuthInvalidCredentials) {
		shared.APIError(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if errors.Is(err, authsvc.ErrAuthUserDisabled) {
		shared.APIError(c, http.StatusForbidden, "user is disabled")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "login failed")
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	c.SetCookie(
		"session_id",
		result.SessionID,
		maxAge,
		"/",
		"",
		secureCookie(c),
		true,
	)

	shared.APIOK(c, http.StatusOK, "login success", gin.H{
		"message":     "login success",
		"user_id":     result.UserID,
		"uuid":        result.UUID,
		"username":    result.Username,
		"permissions": result.Permissions,
	})
}

func (ac *AuthController) Register(c *gin.Context) {
	var registerReq RegisterRequest
	if !bindAuthJSON(c, &registerReq) {
		return
	}

	result, err := ac.authService.Register(registerReq.Username, registerReq.Password)
	if errors.Is(err, authsvc.ErrAuthInvalidInput) {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, authsvc.ErrAuthUsernameExists) {
		shared.APIError(c, http.StatusConflict, "username already exists")
		return
	}
	if errors.Is(err, authsvc.ErrAuthUUIDGeneration) {
		shared.APIError(c, http.StatusInternalServerError, "failed to generate uuid")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "register failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "register success", gin.H{
		"message": "register success",
		"uuid":    result.UUID,
		"user_id": result.UserID,
	})
}

func (ac *AuthController) Logout(c *gin.Context) {
	value, ok := c.Get("sessionId")
	sessionID, valid := value.(string)
	if !ok || !valid || sessionID == "" {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := ac.authService.Logout(sessionID)
	if errors.Is(err, authsvc.ErrAuthSessionNotFound) {
		shared.APIError(c, http.StatusUnauthorized, "invalid session")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "logout failed")
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session_id", "", -1, "/", "", secureCookie(c), true)
	shared.APIOK(c, http.StatusOK, "logout success", gin.H{"message": "logout success"})
}

func secureCookie(c *gin.Context) bool {
	return c.Request.TLS != nil || (config.GlobalConfig != nil && config.GlobalConfig.Server.SecureCookies)
}

func bindAuthJSON(c *gin.Context, value interface{}) bool {
	if c.Request.ContentLength > 8*1024 {
		shared.APIError(c, http.StatusRequestEntityTooLarge, "request body must not exceed 8 KiB")
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024)
	if err := c.ShouldBindJSON(value); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			shared.APIError(c, http.StatusRequestEntityTooLarge, "request body must not exceed 8 KiB")
		} else {
			shared.APIError(c, http.StatusBadRequest, err.Error())
		}
		return false
	}
	return true
}
