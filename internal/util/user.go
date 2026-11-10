package util

import (
	"strings"

	"patreonoj/internal/models"
)

type User struct {
	permissionCheck permissionCheck
}

func (u *User) HasPermission(user models.User, perm string) bool {
	return u.permissionCheck.hasPermission(user, perm)
}

type permissionCheck interface {
	hasPermission(user models.User, perm string) bool
}

type permissionCheckImpl struct{}

// 通过id查权限
func (p *permissionCheckImpl) hasPermission(user models.User, perm string) bool {
	// 在auth时直接直接查好然后放到user，然后直接传user进来就行了
	// var user models.User
	// if err := models.DB.Preload("Roles.Permissions").Where("uuid = ?", uuid).First(&user).Error; err != nil {
	// 	return false
	// }

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
