package admin

import (
	"net/http"

	"patreonoj/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RelationController struct {
	service *services.RelationService
}

func NewRelationController(db *gorm.DB) *RelationController {
	return &RelationController{
		service: services.NewRelationService(db),
	}
}

func (con *RelationController) Index(c *gin.Context) {
	relationList, err := con.service.List()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "获取关系列表失败")
		return
	}
	apiOK(c, http.StatusOK, "ok", gin.H{
		"result": relationList,
	})
}
