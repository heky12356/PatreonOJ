package admin

import (
    "dachuang/models"
    "github.com/gin-gonic/gin"
   
)

type NodeController struct {
}

func (con NodeController) Index(c *gin.Context) {
    nodeList := []models.Node{}
    models.DB.Find(&nodeList)
    c.JSON(200, gin.H{
        "result": nodeList,
    })
}