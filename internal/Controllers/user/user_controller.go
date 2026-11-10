package user

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/models"
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
	Id          *string   `json:"id"` // 前端传id判断修改的用户
	Nickname    *string   `json:"nickname"`
	Email       *string   `json:"email"`
	AvatarURL   *string   `json:"avatar_url"`
	Status      *string   `json:"status"`
	Permissions *string   `json:"permissions"`
	RoleCodes   *[]string `json:"role_codes"`
}

func (uc *UserController) Index(c *gin.Context) {
	// 这是旧的鉴权，后面改用中间件
	// operatorUUID, ok := shared.RequireOperatorUUID(uc.db, c)
	// if !ok {
	// 	return
	// }
	user, ok := c.Get("currentUser")
	if !ok {
		shared.APIError(c, http.StatusForbidden, "未授权")
		return
	}
	if !util.UserInstance.HasPermission(user.(models.User), "admin") {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}

	userList, err := uc.service.List(usersvc.UserListFilter{
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
		RoleCode: c.Query("role_code"),
	})
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query users failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": userList})
}

func (uc *UserController) Show(c *gin.Context) {
	id := c.Param("id")
	user, ok := c.Get("currentUser")
	if !ok {
		shared.APIError(c, http.StatusForbidden, "未登录")
		return
	}
	target, err := uc.service.GetByIdentifier(id)
	if errors.Is(err, usersvc.ErrUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query user failed")
		return
	}
	if !shared.CanAccessUserState(user.(models.User), strconv.Itoa(target.Id)) {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": target})
}

func (uc *UserController) GetProfilePublic(c *gin.Context) {
	id := c.Param("id")

	user, err := uc.service.GetProfilePublic(id)
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

func (uc *UserController) GetProfile(c *gin.Context) {
	user, ok := c.Get("currentUser")
	if !ok {
		shared.APIError(c, http.StatusForbidden, "user not login")
		return
	}

	user, err := uc.service.GetProfileById(strconv.Itoa(user.(models.User).Id))
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
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	target, err := uc.service.GetByIdentifier(strings.TrimSpace(c.Param("id")))
	if errors.Is(err, usersvc.ErrUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query user failed")
		return
	}
	targetNumber := target.Id
	targetID := strconv.Itoa(targetNumber)
	if req.Id != nil {
		bodyID, err := strconv.Atoi(strings.TrimSpace(*req.Id))
		if err != nil || bodyID != targetNumber {
			shared.APIError(c, http.StatusBadRequest, "body id must match the user id in the URL")
			return
		}
	}

	user, ok := c.Get("currentUser")
	if !ok {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !shared.CanAccessUserState(user.(models.User), targetID) {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}

	isAdmin := util.UserInstance.HasPermission(user.(models.User), "admin")
	if !isAdmin && (req.Status != nil || req.Permissions != nil || req.RoleCodes != nil) {
		shared.APIError(c, http.StatusForbidden, "only admin can update status, permissions and roles")
		return
	}

	user, err = uc.service.UpdateByID(targetID, usersvc.UserUpdateInput{
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
