package admin

import (
	"net/http"

	"patreonoj/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NodeController struct {
	service *services.NodeService
}

func NewNodeController(db *gorm.DB) *NodeController {
	return &NodeController{
		service: services.NewNodeService(db),
	}
}

func (con *NodeController) Index(c *gin.Context) {
	nodeList, err := con.service.List()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "获取节点列表失败")
		return
	}
	apiOK(c, http.StatusOK, "ok", gin.H{
		"result": nodeList,
	})
}
