package graph

import (
	"net/http"

	"patreonoj/internal/controllers/shared"
	graphsvc "patreonoj/internal/services/graph"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NodeController struct {
	service *graphsvc.NodeService
}

func NewNodeController(db *gorm.DB) *NodeController {
	return &NodeController{service: graphsvc.NewNodeService(db)}
}

func (con *NodeController) Index(c *gin.Context) {
	nodeList, err := con.service.List()
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query nodes failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": nodeList})
}
