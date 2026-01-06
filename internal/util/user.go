package util

import (
	"strings"

	"dachuang/internal/models"
)

type User struct {
	permissionCheck permissionCheck
}

func (u *User) HasPermission(uuid string, perm string) bool {
	return u.permissionCheck.hasPermission(uuid, perm)
}

type permissionCheck interface {
	hasPermission(uuid string, perm string) bool
}

type permissionCheckImpl struct{}

func (p *permissionCheckImpl) hasPermission(uuid string, perm string) bool {
	if uuid == "" {
		return false
	}

	var user models.User
	if err := models.DB.Preload("Roles.Permissions").Where("uuid = ?", uuid).First(&user).Error; err != nil {
		return false
	}

	if user.Status != "" && user.Status != "active" {
		return false
	}

	if perm == "" {
		return true // 空权限默认允许
	}

	if user.Permissions != "" {
		for _, token := range strings.Split(user.Permissions, ",") {
			if strings.TrimSpace(token) == perm {
				return true
			}
		}
	}

	for _, role := range user.Roles {
		for _, p := range role.Permissions {
			if p.Code == perm {
				return true
			}
		}
	}

	return false
}

var UserInstance = &User{
	permissionCheck: &permissionCheckImpl{},
}
