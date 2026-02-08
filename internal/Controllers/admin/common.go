package admin

import (
	"net/http"
	"strings"

	"dachuang/internal/models"
	"dachuang/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// canAccessUserState 检查操作者是否有权限访问目标用户的状态
func canAccessUserState(operatorUUID, targetUUID string) bool {
	if operatorUUID == targetUUID {
		return true
	}
	return util.UserInstance.HasPermission(operatorUUID, "admin")
}

// requireOperatorUUID 从请求头或查询参数中获取操作人UUID并验证
func requireOperatorUUID(db *gorm.DB, c *gin.Context) (string, bool) {
	op := strings.TrimSpace(c.GetHeader("X-User-UUID"))
	if op == "" {
		op = strings.TrimSpace(c.Query("operator_uuid"))
	}
	if op == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return "", false
	}

	var user models.User
	if err := db.Where("uuid = ?", op).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的登录状态"})
		return "", false
	}
	if user.Status != "" && user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return "", false
	}
	return op, true
}
