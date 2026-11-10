package learning

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/models"
	learningsvc "patreonoj/internal/services/learning"
	"patreonoj/internal/util"

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
	admin, ok := authorizeLearningUser(c, userUUID)
	if !ok {
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

	result, err := lc.service.WithDrafts(admin).RecommendNextQuestions(userUUID, questionID, limit)
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
	if _, ok := authorizeLearningUser(c, userUUID); !ok {
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

	admin := false
	if value, ok := c.Get("currentUser"); ok {
		if user, valid := value.(models.User); valid {
			admin = util.UserInstance.HasPermission(user, "admin")
		}
	}
	result, err := lc.service.WithDrafts(admin).BuildQuestionGraph(questionID, depth)
	if errors.Is(err, learningsvc.ErrLearningAmbiguousQuestion) {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}
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

func authorizeLearningUser(c *gin.Context, targetUUID string) (bool, bool) {
	value, exists := c.Get("currentUser")
	user, valid := value.(models.User)
	if !exists || !valid {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return false, false
	}
	admin := util.UserInstance.HasPermission(user, "admin")
	if user.UUID != targetUUID && !admin {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return false, false
	}
	return admin, true
}
