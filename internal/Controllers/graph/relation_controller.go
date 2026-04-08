package graph

import (
	"net/http"

	"patreonoj/internal/controllers/shared"
	graphsvc "patreonoj/internal/services/graph"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RelationController struct {
	service *graphsvc.RelationService
}

func NewRelationController(db *gorm.DB) *RelationController {
	return &RelationController{service: graphsvc.NewRelationService(db)}
}

func (con *RelationController) Index(c *gin.Context) {
	relationList, err := con.service.List()
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query relations failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": relationList})
}
