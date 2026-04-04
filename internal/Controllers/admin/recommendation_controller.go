package admin

import (
	"net/http"

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
		apiError(ctx, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	limit, err := parsePositiveIntQuery(ctx, "limit", 10)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err.Error())
		return
	}
	if limit > 50 {
		limit = 50
	}

	recommendations, err := c.RecommendationService.GetPersonalizedRecommendations(userUUID, limit)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	apiOK(ctx, http.StatusOK, "ok", gin.H{"data": recommendations})
}
