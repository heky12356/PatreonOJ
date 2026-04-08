package shared

import (
	"strings"

	"patreonoj/internal/models"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CanAccessUserState(operatorUUID, targetUUID string) bool {
	if operatorUUID == targetUUID {
		return true
	}
	return util.UserInstance.HasPermission(operatorUUID, "admin")
}

func RequireOperatorUUID(db *gorm.DB, c *gin.Context) (string, bool) {
	op := strings.TrimSpace(c.GetHeader("X-User-UUID"))
	if op == "" {
		op = strings.TrimSpace(c.Query("operator_uuid"))
	}
	if op == "" {
		APIError(c, 401, "unauthorized")
		return "", false
	}

	var user models.User
	if err := db.Where("uuid = ?", op).First(&user).Error; err != nil {
		APIError(c, 401, "invalid session")
		return "", false
	}
	if user.Status != "" && user.Status != "active" {
		APIError(c, 403, "user is disabled")
		return "", false
	}
	return op, true
}
