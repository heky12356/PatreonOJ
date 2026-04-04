package admin

import (
	"errors"
	"net/http"

	"patreonoj/internal/services"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserController handles user profile endpoints.
type UserController struct {
	db      *gorm.DB
	service *services.UserService
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{
		db:      db,
		service: services.NewUserService(db),
	}
}

type UpdateUserRequest struct {
	Nickname    *string  `json:"nickname"`
	Email       *string  `json:"email"`
	AvatarURL   *string  `json:"avatar_url"`
	Status      *string  `json:"status"`
	Permissions *string  `json:"permissions"`
	RoleCodes   []string `json:"role_codes"`
}

func (uc *UserController) Index(c *gin.Context) {
	userList, err := uc.service.List()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询用户列表失败")
		return
	}
	apiOK(c, http.StatusOK, "ok", gin.H{"result": userList})
}

func (uc *UserController) Show(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		apiError(c, http.StatusForbidden, "无权限")
		return
	}

	user, err := uc.service.GetByUUID(targetUUID)
	if errors.Is(err, services.ErrUserNotFound) {
		apiError(c, http.StatusNotFound, "用户不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询用户失败")
		return
	}
	apiOK(c, http.StatusOK, "ok", gin.H{"result": user})
}

func (uc *UserController) Update(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		apiError(c, http.StatusForbidden, "无权限")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	isAdmin := util.UserInstance.HasPermission(operatorUUID, "admin")
	if !isAdmin && (req.Status != nil || req.Permissions != nil || len(req.RoleCodes) > 0) {
		apiError(c, http.StatusForbidden, "仅管理员可修改权限、角色和状态")
		return
	}

	user, err := uc.service.UpdateByUUID(targetUUID, services.UserUpdateInput{
		Nickname:    req.Nickname,
		Email:       req.Email,
		AvatarURL:   req.AvatarURL,
		Status:      req.Status,
		Permissions: req.Permissions,
		RoleCodes:   req.RoleCodes,
	})
	if errors.Is(err, services.ErrUserNotFound) {
		apiError(c, http.StatusNotFound, "用户不存在")
		return
	}
	if errors.Is(err, services.ErrUserStatusInvalid) {
		apiError(c, http.StatusBadRequest, "status 仅支持 active/disabled")
		return
	}
	if errors.Is(err, services.ErrUserRoleNotFound) {
		apiError(c, http.StatusBadRequest, "role_codes 包含不存在的角色")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "更新失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{"result": user})
}
