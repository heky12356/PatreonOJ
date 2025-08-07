package admin

import (
    "dachuang/internal/models"
    "github.com/gin-gonic/gin"
   
)

type RelationController struct {
}

func (con RelationController) Index(c *gin.Context) {
    relationList := []models.Relation{}
    models.DB.Find(&relationList)
    c.JSON(200, gin.H{
        "result": relationList,
    })
}