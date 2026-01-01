package util

import (
	"fmt"
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
	models.DB.Where("uuid = ?", uuid).First(&user)
	fmt.Println("uuid = " + uuid + "permission : " + user.Permissions)
	if strings.Contains(user.Permissions, perm) {
		return true
	} else {
		fmt.Println("没有权限")
		return false
	}
}

var UserInstance = &User{
	permissionCheck: &permissionCheckImpl{},
}
