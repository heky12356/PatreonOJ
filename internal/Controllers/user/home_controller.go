package user

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/models"
	usersvc "patreonoj/internal/services/user"
)

func (uc *UserController) Home(c *gin.Context) {
	page, size, err := shared.ParsePageSizeQuery(c, "page", "size", 1, 20, 100)
	if err != nil {
		shared.APIError(c, 400, err.Error())
		return
	}
	viewer := models.User{}
	if value, ok := c.Get("currentUser"); ok {
		viewer, _ = value.(models.User)
	}
	home, err := uc.service.GetHome(c.Param("id"), viewer, page, size)
	if errors.Is(err, usersvc.ErrUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		shared.APIError(c, 500, "query user home failed")
		return
	}
	shared.APIOK(c, 200, "ok", gin.H{"result": home})
}
