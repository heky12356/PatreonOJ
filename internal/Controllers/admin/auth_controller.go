package admin

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"

	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthController 处理用户认证相关的请求
type AuthController struct {
	db *gorm.DB
}

// NewAuthController 创建认证控制器
func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{db: db}
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

// md5Encode MD5加密函数
func md5Encode(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// Login 用户登录
func (ac *AuthController) Login(c *gin.Context) {
	var loginReq LoginRequest
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	encryptedPassword := md5Encode(loginReq.Password)

	var user models.User
	result := ac.db.Where("username = ? AND password = ?", loginReq.Username, encryptedPassword).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if user.Status != "" && user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return
	}

	permissions := strings.Split(user.Permissions, ",")

	c.JSON(http.StatusOK, gin.H{
		"message":     "登录成功",
		"user_id":     user.Id,
		"uuid":        user.UUID,
		"username":    user.Username,
		"permissions": permissions,
	})
}

// Register 用户注册
func (ac *AuthController) Register(c *gin.Context) {
	var registerReq RegisterRequest
	if err := c.ShouldBindJSON(&registerReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existingUser models.User
	result := ac.db.Where("username = ?", registerReq.Username).First(&existingUser)
	if result.Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}

	var userUUID string
	var uuidExists bool
	for {
		userUUID = uuid.New().String()
		result := ac.db.Where("uuid = ?", userUUID).First(&existingUser)
		if result.Error != nil {
			uuidExists = false
			break
		}
		uuidExists = true
	}

	if uuidExists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法生成唯一UUID"})
		return
	}

	encryptedPassword := md5Encode(registerReq.Password)

	newUser := models.User{
		Username: registerReq.Username,
		Password: encryptedPassword,
		UUID:     userUUID,
	}

	if err := ac.db.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"uuid":    userUUID,
		"user_id": newUser.Id,
	})
}

// Logout 用户登出
func (ac *AuthController) Logout(c *gin.Context) {
	var logoutReq LogoutRequest
	if err := c.ShouldBindJSON(&logoutReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	result := ac.db.First(&user, logoutReq.UserID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	user.UUID = ""
	ac.db.Save(&user)

	c.JSON(http.StatusOK, gin.H{"message": "注销成功"})
}
