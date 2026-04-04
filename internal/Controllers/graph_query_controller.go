package Controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
)

// GetPrerequisites 获取前置题目
func (gc *GraphController) GetPrerequisites(c *gin.Context) {
	questionNumber, ok := graphParamInt(c, "number", "无效的题目编号")
	if !ok {
		return
	}

	ctx := context.Background()
	prerequisites, err := gc.queryService.GetPrerequisites(ctx, questionNumber)
	if err != nil {
		graphJSONError(c, http.StatusInternalServerError, "获取前置题目失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question_number": questionNumber,
		"prerequisites":   prerequisites,
	})
}

// GetNextLevelQuestions 获取进阶题目
func (gc *GraphController) GetNextLevelQuestions(c *gin.Context) {
	questionNumber, ok := graphParamInt(c, "number", "无效的题目编号")
	if !ok {
		return
	}

	ctx := context.Background()
	nextQuestions, err := gc.queryService.GetNextLevelQuestions(ctx, questionNumber)
	if err != nil {
		graphJSONError(c, http.StatusInternalServerError, "获取进阶题目失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question_number": questionNumber,
		"next_questions":  nextQuestions,
	})
}

// FindLearningPath 查找学习路径
func (gc *GraphController) FindLearningPath(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr == "" || endStr == "" {
		graphJSONError(c, http.StatusBadRequest, "请提供起始题目和目标题目编号")
		return
	}

	startQuestion, err := strconv.Atoi(startStr)
	if err != nil {
		graphJSONError(c, http.StatusBadRequest, "无效的起始题目编号")
		return
	}

	endQuestion, err := strconv.Atoi(endStr)
	if err != nil {
		graphJSONError(c, http.StatusBadRequest, "无效的目标题目编号")
		return
	}

	ctx := context.Background()
	path, err := gc.queryService.FindLearningPath(ctx, startQuestion, endQuestion)
	if err != nil {
		graphJSONError(c, http.StatusInternalServerError, "查找学习路径失败")
		return
	}

	if path == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "未找到学习路径"})
		return
	}

	c.JSON(http.StatusOK, path)
}

// GetRecommendations 获取推荐题目
func (gc *GraphController) GetRecommendations(c *gin.Context) {
	questionNumber, ok := graphParamInt(c, "number", "无效的题目编号")
	if !ok {
		return
	}

	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}

	ctx := context.Background()
	recommendations, err := gc.queryService.GetRecommendations(ctx, questionNumber, limit)
	if err != nil {
		graphJSONError(c, http.StatusInternalServerError, "获取推荐题目失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question_number": questionNumber,
		"recommendations": recommendations,
	})
}

// ListQuestions 获取图节点（题目 + 技能）及边（用于前端可视化）
func (gc *GraphController) ListQuestions(c *gin.Context) {
	ctx := context.Background()
	result, err := gc.queryService.ListGraphData(ctx)
	if err != nil {
		var qerr *services.GraphQueryError
		if errors.As(err, &qerr) {
			switch qerr.Stage {
			case services.GraphListStageQuestions:
				graphJSONError(c, http.StatusInternalServerError, "获取题目节点失败")
			case services.GraphListStageSkills:
				graphJSONError(c, http.StatusInternalServerError, "获取技能节点失败")
			case services.GraphListStageQuestionRelations:
				graphJSONError(c, http.StatusInternalServerError, "获取题目关系失败")
			case services.GraphListStageQuestionSkillRelations:
				graphJSONError(c, http.StatusInternalServerError, "获取题目-技能关系失败")
			case services.GraphListStageSkillRelations:
				graphJSONError(c, http.StatusInternalServerError, "获取技能关系失败")
			default:
				graphJSONError(c, http.StatusInternalServerError, "获取图数据失败")
			}
			return
		}
		graphJSONError(c, http.StatusInternalServerError, "获取图数据失败")
		return
	}

	c.JSON(http.StatusOK, result)
}
