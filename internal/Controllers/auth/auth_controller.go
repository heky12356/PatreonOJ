package auth

import (
	"errors"
	"net/http"
	"time"

	"patreonoj/internal/controllers/shared"
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
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := ac.authService.Login(loginReq.Username, loginReq.Password)
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
		false,
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
	if err := c.ShouldBindJSON(&registerReq); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := ac.authService.Register(registerReq.Username, registerReq.Password)
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
	sessionId, ok := c.Get("sessionId")
	if !ok {
		shared.APIError(c, http.StatusForbidden, "未登录？")
		return
	}

	err := ac.authService.Logout(sessionId.(string))
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "logout failed")
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session_id", "", -1, "/", "", false, true)
	shared.APIOK(c, http.StatusOK, "logout success", gin.H{"message": "logout success"})
}
