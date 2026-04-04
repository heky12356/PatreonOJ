package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"

	"patreonoj/internal/services"

	"github.com/gin-gonic/gin"
)

// SyncQuestion 同步题目到图数据库
func (gc *GraphController) SyncQuestion(c *gin.Context) {
	questionNumber, ok := graphParamInt(c, "number", "无效的题目编号")
	if !ok {
		return
	}

	ctx := context.Background()
	err := gc.writeService.SyncQuestion(ctx, questionNumber)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "题目、标签及同标签关系同步成功"})
		return
	}

	if errors.Is(err, services.ErrSyncQuestionNotFound) {
		graphJSONError(c, http.StatusNotFound, "题目不存在")
		return
	}

	var syncErr *services.GraphSyncError
	if errors.As(err, &syncErr) {
		log.Printf("同步题目失败, stage=%s err=%v", syncErr.Stage, syncErr.Err)
		switch syncErr.Stage {
		case services.GraphSyncStageQueryQuestion:
			graphJSONError(c, http.StatusInternalServerError, "查询题目失败")
		case services.GraphSyncStageUpsertQuestion:
			graphJSONError(c, http.StatusInternalServerError, "同步题目到图数据库失败")
		case services.GraphSyncStageSyncSkills:
			graphJSONError(c, http.StatusInternalServerError, "同步题目标签失败")
		case services.GraphSyncStageBuildSimilarEdge:
			graphJSONError(c, http.StatusInternalServerError, "构建同标签题目关系失败")
		default:
			graphJSONError(c, http.StatusInternalServerError, "同步题目失败")
		}
		return
	}

	log.Printf("同步题目失败: %v", err)
	graphJSONError(c, http.StatusInternalServerError, "同步题目失败")
}
