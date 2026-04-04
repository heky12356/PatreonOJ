package admin

import (
	"errors"
	"net/http"

	"patreonoj/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuthController 处理用户认证相关的请求
type AuthController struct {
	db          *gorm.DB
	authService *services.AuthService
}

// NewAuthController 创建认证控制器
func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{
		db:          db,
		authService: services.NewAuthService(db),
	}
}

// LoginRequest 定义登录请求结构体
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 定义注册请求结构体
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LogoutRequest 定义登出请求结构体
type LogoutRequest struct {
	UserID int `json:"user_id" binding:"required"`
}

// Login 用户登录
func (ac *AuthController) Login(c *gin.Context) {
	var loginReq LoginRequest
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := ac.authService.Login(loginReq.Username, loginReq.Password)
	if errors.Is(err, services.ErrAuthInvalidCredentials) {
		apiError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if errors.Is(err, services.ErrAuthUserDisabled) {
		apiError(c, http.StatusForbidden, "用户已被禁用")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "登录失败")
		return
	}
	apiOK(c, http.StatusOK, "登录成功", gin.H{
		"message":     "登录成功",
		"user_id":     result.UserID,
		"uuid":        result.UUID,
		"username":    result.Username,
		"permissions": result.Permissions,
	})
}

// Register 用户注册
func (ac *AuthController) Register(c *gin.Context) {
	var registerReq RegisterRequest
	if err := c.ShouldBindJSON(&registerReq); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := ac.authService.Register(registerReq.Username, registerReq.Password)
	if errors.Is(err, services.ErrAuthUsernameExists) {
		apiError(c, http.StatusConflict, "用户名已存在")
		return
	}
	if errors.Is(err, services.ErrAuthUUIDGeneration) {
		apiError(c, http.StatusInternalServerError, "无法生成唯一UUID")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "注册失败")
		return
	}
	apiOK(c, http.StatusOK, "注册成功", gin.H{
		"message": "注册成功",
		"uuid":    result.UUID,
		"user_id": result.UserID,
	})
}

// Logout 用户登出
func (ac *AuthController) Logout(c *gin.Context) {
	var logoutReq LogoutRequest
	if err := c.ShouldBindJSON(&logoutReq); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	err := ac.authService.Logout(logoutReq.UserID)
	if errors.Is(err, services.ErrAuthUserNotFound) {
		apiError(c, http.StatusNotFound, "用户不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "注销失败")
		return
	}
	apiOK(c, http.StatusOK, "注销成功", gin.H{"message": "注销成功"})
}
