package admin

import (
	"errors"
	"net/http"
	"strings"

	"dachuang/internal/graph"
	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
)

// StatsController 处理用户统计相关的请求
type StatsController struct {
	service *services.StatsService
}

// NewStatsController 创建统计控制器
func NewStatsController(graphService *graph.QuestionGraphService) *StatsController {
	return &StatsController{
		service: services.NewStatsService(graphService),
	}
}

// GetUserRadarStats 获取用户能力雷达图数据
func (sc *StatsController) GetUserRadarStats(c *gin.Context) {
	userUUID := strings.TrimSpace(c.Query("user_uuid"))
	if userUUID == "" {
		apiError(c, http.StatusBadRequest, "user_uuid 必填")
		return
	}

	stats, err := sc.service.BuildUserRadarStats(userUUID, 6)
	if errors.Is(err, services.ErrStatsGraphUnavailable) {
		apiError(c, http.StatusInternalServerError, "图服务未初始化")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "获取能力数据失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"data": stats,
	})
}
