package admin

import (
	"net/http"
	"strconv"

	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
)

type RecommendationController struct {
	RecommendationService *services.RecommendationService
}

func NewRecommendationController(service *services.RecommendationService) *RecommendationController {
	return &RecommendationController{
		RecommendationService: service,
	}
}

// GetRecommendations 获取个性化推荐题目
func (c *RecommendationController) GetRecommendations(ctx *gin.Context) {
	// 获取当前登录用户uuid
	userUUID := ctx.Query("uuid")

	if userUUID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	limitStr := ctx.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	recommendations, err := c.RecommendationService.GetPersonalizedRecommendations(userUUID, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": recommendations,
	})
}
