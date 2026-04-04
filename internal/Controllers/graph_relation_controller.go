package Controllers

import (
	"context"
	"errors"
	"net/http"

	"dachuang/internal/graph"
	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
)

// CreateRelation 创建题目关系
func (gc *GraphController) CreateRelation(c *gin.Context) {
	var req struct {
		FromQuestion int                    `json:"from_question" binding:"required"`
		ToQuestion   int                    `json:"to_question" binding:"required"`
		RelationType graph.RelationshipType `json:"relation_type" binding:"required"`
		Weight       float64                `json:"weight"`
		Description  string                 `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		graphJSONError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := context.Background()
	err := gc.writeService.CreateRelation(ctx, req.FromQuestion, req.ToQuestion, req.RelationType, req.Weight, req.Description)
	if errors.Is(err, services.ErrInvalidRelationType) {
		graphJSONError(c, http.StatusBadRequest, "无效的关系类型")
		return
	}
	if err != nil {
		graphJSONError(c, http.StatusInternalServerError, "创建关系失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "关系创建成功"})
}

// DeleteRelation 删除题目关系
func (gc *GraphController) DeleteRelation(c *gin.Context) {
	var req struct {
		FromQuestion int                    `json:"from_question" binding:"required"`
		ToQuestion   int                    `json:"to_question" binding:"required"`
		RelationType graph.RelationshipType `json:"relation_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		graphJSONError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := context.Background()
	if err := gc.writeService.DeleteRelation(ctx, req.FromQuestion, req.ToQuestion, req.RelationType); err != nil {
		graphJSONError(c, http.StatusInternalServerError, "删除关系失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "关系删除成功"})
}
