package user

import (
	"errors"
	"net/http"

	"patreonoj/internal/controllers/shared"
	usersvc "patreonoj/internal/services/user"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserController handles user profile endpoints.
type UserController struct {
	db      *gorm.DB
	service *usersvc.UserService
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{
		db:      db,
		service: usersvc.NewUserService(db),
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
		shared.APIError(c, http.StatusInternalServerError, "query users failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": userList})
}

func (uc *UserController) Show(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !shared.CanAccessUserState(operatorUUID, targetUUID) {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}

	user, err := uc.service.GetByUUID(targetUUID)
	if errors.Is(err, usersvc.ErrUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query user failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": user})
}

func (uc *UserController) Update(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !shared.CanAccessUserState(operatorUUID, targetUUID) {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	isAdmin := util.UserInstance.HasPermission(operatorUUID, "admin")
	if !isAdmin && (req.Status != nil || req.Permissions != nil || len(req.RoleCodes) > 0) {
		shared.APIError(c, http.StatusForbidden, "only admin can update status, permissions and roles")
		return
	}

	user, err := uc.service.UpdateByUUID(targetUUID, usersvc.UserUpdateInput{
		Nickname:    req.Nickname,
		Email:       req.Email,
		AvatarURL:   req.AvatarURL,
		Status:      req.Status,
		Permissions: req.Permissions,
		RoleCodes:   req.RoleCodes,
	})
	if errors.Is(err, usersvc.ErrUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if errors.Is(err, usersvc.ErrUserStatusInvalid) {
		shared.APIError(c, http.StatusBadRequest, "status must be active or disabled")
		return
	}
	if errors.Is(err, usersvc.ErrUserRoleNotFound) {
		shared.APIError(c, http.StatusBadRequest, "role_codes contains unknown role")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "update user failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": user})
}
