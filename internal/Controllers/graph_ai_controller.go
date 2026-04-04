package Controllers

import (
	"context"
	"errors"
	"log"
	"net/http"

	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
)

// AnalyzeQuestionRelations 调用 AI 分析题目关系
func (gc *GraphController) AnalyzeQuestionRelations(c *gin.Context) {
	questionNumber, ok := graphParamInt(c, "number", "无效的题目编号")
	if !ok {
		return
	}

	ctx := context.Background()
	relations, saved, err := gc.analysisService.AnalyzeQuestionRelations(ctx, questionNumber)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			graphJSONError(c, http.StatusNotFound, "题目不存在")
			return
		}
		log.Printf("AI分析失败: %v", err)
		graphJSONError(c, http.StatusInternalServerError, "AI分析失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "分析完成",
		"relations": relations,
		"saved":     saved,
	})
}

// AnalyzeSkillTree 调用 AI 构建技能树
func (gc *GraphController) AnalyzeSkillTree(c *gin.Context) {
	ctx := context.Background()

	relations, saved, failed, hasSkills, err := gc.analysisService.AnalyzeSkillTree(ctx)
	if err != nil {
		graphJSONError(c, http.StatusInternalServerError, "AI分析失败: "+err.Error())
		return
	}

	if !hasSkills {
		c.JSON(http.StatusOK, gin.H{"message": "暂无技能数据", "relations": []interface{}{}, "saved": 0})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "技能树分析完成",
		"relations": relations,
		"saved":     saved,
		"failed":    failed,
	})
}
