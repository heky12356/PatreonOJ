package admin

import (
	"context"
	"net/http"
	"sort"

	"dachuang/internal/graph"

	"github.com/gin-gonic/gin"
)

// StatsController 处理用户统计相关的请求
type StatsController struct {
	graphService *graph.QuestionGraphService
}

// NewStatsController 创建统计控制器
func NewStatsController(graphService *graph.QuestionGraphService) *StatsController {
	return &StatsController{graphService: graphService}
}

// RadarStatItem 雷达图数据项
type RadarStatItem struct {
	Subject  string  `json:"subject"`
	Value    float64 `json:"A"`        // 为了适配某些图表库习惯用 A/B
	FullMark float64 `json:"fullMark"` // 该维度的最大值上限，这里恒为100
}

// GetUserRadarStats 获取用户能力雷达图数据
func (sc *StatsController) GetUserRadarStats(c *gin.Context) {
	// 获取当前登录用户ID
	userUUID := c.Query("user_uuid")
	if userUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_uuid 必填"})
		return
	}

	ctx := context.Background()
	masteries, err := sc.graphService.GetUserMastery(ctx, userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取能力数据失败"})
		return
	}

	// 按掌握度降序取前 6 个，如果不足 3 个则补全或直接返回
	sort.Slice(masteries, func(i, j int) bool {
		return masteries[i].Mastery > masteries[j].Mastery
	})

	limit := 6
	if len(masteries) > limit {
		masteries = masteries[:limit]
	}

	stats := make([]RadarStatItem, 0, len(masteries))
	for _, m := range masteries {
		stats = append(stats, RadarStatItem{
			Subject:  m.SkillKey,
			Value:    m.Mastery * 100,
			FullMark: 100,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": stats,
	})
}
