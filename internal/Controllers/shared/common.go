package shared

import (
	"net/http"
	"strconv"

	"patreonoj/internal/models"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CanAccessUserState(user models.User, targetID string) bool {
	id, err := strconv.Atoi(targetID)
	if err != nil || id <= 0 || user.Id <= 0 || (user.Status != "" && user.Status != "active") {
		return false
	}
	isAdmin := util.UserInstance.HasPermission(user, "admin")
	if user.Id != id && !isAdmin {
		return false
	}

	return true
}

// RequireOperatorUUID uses the authenticated session; request headers and query
// parameters cannot establish an operator's identity.
func RequireOperatorUUID(_ *gorm.DB, c *gin.Context) (string, bool) {
	value, ok := c.Get("currentUser")
	user, valid := value.(models.User)
	if !ok || !valid || user.Id <= 0 || user.UUID == "" {
		APIError(c, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	if user.Status != "" && user.Status != "active" {
		APIError(c, 403, "user is disabled")
		return "", false
	}
	return user.UUID, true
}
