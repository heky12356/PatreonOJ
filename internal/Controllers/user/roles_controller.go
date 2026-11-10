package user

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/models"
	"patreonoj/internal/util"
)

func (uc *UserController) Roles(c *gin.Context) {
	value, ok := c.Get("currentUser")
	viewer, valid := value.(models.User)
	if !ok || !valid {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !util.UserInstance.HasPermission(viewer, "admin") {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}
	roles := []models.Role{}
	if err := uc.db.Preload("Permissions").Order("id ASC").Find(&roles).Error; err != nil {
		shared.APIError(c, 500, "query roles failed")
		return
	}
	shared.APIOK(c, 200, "ok", gin.H{"result": roles})
}
