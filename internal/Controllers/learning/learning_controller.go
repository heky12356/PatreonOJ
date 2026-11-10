package learning

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"patreonoj/internal/controllers/shared"
	learningsvc "patreonoj/internal/services/learning"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LearningController struct {
	service *learningsvc.LearningService
}

func NewLearningController(db *gorm.DB) *LearningController {
	return &LearningController{
		service: learningsvc.NewLearningService(db),
	}
}

func (lc *LearningController) RecommendByQuestion(c *gin.Context) {
	userUUID := strings.TrimSpace(c.Param("uuid"))
	if userUUID == "" {
		shared.APIError(c, http.StatusBadRequest, "missing user uuid")
		return
	}

	questionID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || questionID <= 0 {
		shared.APIError(c, http.StatusBadRequest, "invalid question id")
		return
	}

	limit, err := shared.ParsePositiveIntQuery(c, "limit", 5)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}
	if limit > 20 {
		limit = 20
	}

	result, err := lc.service.RecommendNextQuestions(userUUID, questionID, limit)
	if errors.Is(err, learningsvc.ErrLearningUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if errors.Is(err, learningsvc.ErrLearningQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "build recommendation failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{"data": result})
}

func (lc *LearningController) GetUserFeedback(c *gin.Context) {
	userUUID := strings.TrimSpace(c.Param("uuid"))
	if userUUID == "" {
		shared.APIError(c, http.StatusBadRequest, "missing user uuid")
		return
	}

	limit, err := shared.ParsePositiveIntQuery(c, "limit", 5)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}
	if limit > 20 {
		limit = 20
	}

	result, err := lc.service.BuildUserFeedback(userUUID, limit)
	if errors.Is(err, learningsvc.ErrLearningUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "build feedback failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{"data": result})
}

func (lc *LearningController) GetQuestionGraph(c *gin.Context) {
	questionID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || questionID <= 0 {
		shared.APIError(c, http.StatusBadRequest, "invalid question id")
		return
	}

	depth, err := shared.ParsePositiveIntQuery(c, "depth", 3)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := lc.service.BuildQuestionGraph(questionID, depth)
	if errors.Is(err, learningsvc.ErrLearningQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "build graph failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{"data": result})
}
