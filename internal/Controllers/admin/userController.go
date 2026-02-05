package admin

import (
	"net/http"
	"strings"

	"dachuang/internal/models"
	"dachuang/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserController 处理用户基础信息相关的请求
type UserController struct {
	db *gorm.DB
}

// NewUserController 创建用户控制器
func NewUserController(db *gorm.DB) *UserController {
	return &UserController{db: db}
}

// UpdateUserRequest 更新用户请求结构体
type UpdateUserRequest struct {
	Nickname    *string  `json:"nickname"`
	Email       *string  `json:"email"`
	AvatarURL   *string  `json:"avatar_url"`
	Status      *string  `json:"status"`
	Permissions *string  `json:"permissions"`
	RoleCodes   []string `json:"role_codes"`
}

// Index 获取用户列表
func (uc *UserController) Index(c *gin.Context) {
	userList := []models.User{}
	uc.db.Find(&userList)
	c.JSON(200, gin.H{
		"result": userList,
	})
}

// Show 获取用户详情
func (uc *UserController) Show(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	var user models.User
	if err := uc.db.Preload("Roles.Permissions").Where("uuid = ?", targetUUID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": user})
}

// Update 更新用户信息
func (uc *UserController) Update(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isAdmin := util.UserInstance.HasPermission(operatorUUID, "admin")
	if !isAdmin {
		if req.Status != nil || req.Permissions != nil || len(req.RoleCodes) > 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可修改权限/角色/状态"})
			return
		}
	}

	var user models.User
	if err := uc.db.Preload("Roles").Where("uuid = ?", targetUUID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	updates := map[string]any{}
	if req.Nickname != nil {
		updates["nickname"] = strings.TrimSpace(*req.Nickname)
	}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = strings.TrimSpace(*req.AvatarURL)
	}
	if req.Status != nil {
		s := strings.TrimSpace(*req.Status)
		if s != "active" && s != "disabled" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status 仅支持 active/disabled"})
			return
		}
		updates["status"] = s
	}
	if req.Permissions != nil {
		updates["permissions"] = strings.TrimSpace(*req.Permissions)
	}

	if len(updates) > 0 {
		if err := uc.db.Model(&user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
	}

	if len(req.RoleCodes) > 0 {
		roles := make([]models.Role, 0, len(req.RoleCodes))
		if err := uc.db.Where("code IN ?", req.RoleCodes).Find(&roles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询角色失败"})
			return
		}
		if len(roles) != len(req.RoleCodes) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role_codes 包含不存在的角色"})
			return
		}
		if err := uc.db.Model(&user).Association("Roles").Replace(&roles); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "绑定角色失败"})
			return
		}
	}

	_ = uc.db.Preload("Roles.Permissions").Where("uuid = ?", targetUUID).First(&user)
	c.JSON(http.StatusOK, gin.H{"result": user})
}
