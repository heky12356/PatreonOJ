package learning

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"patreonoj/internal/graphcache"
	"strconv"
	"strings"
	"time"

	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/models"
	learningsvc "patreonoj/internal/services/learning"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LearningController struct {
	service   *learningsvc.LearningService
	responses *graphcache.Cache
}

func NewLearningController(db *gorm.DB) *LearningController {
	return &LearningController{
		service:   learningsvc.NewLearningService(db),
		responses: graphcache.New(),
	}
}

func (lc *LearningController) Tags(c *gin.Context) {
	lc.graphResponse(c, "tags", func() (any, error) { return lc.service.TagCatalog() })
}

func (lc *LearningController) TagGraph(c *gin.Context) {
	tag := strings.TrimSpace(c.Query("tag"))
	query := strings.TrimSpace(c.Query("q"))
	if tag == "" || len([]rune(tag)) > 128 || len([]rune(query)) > 200 {
		shared.APIError(c, 400, "请选择有效知识点")
		return
	}
	page, size, err := shared.ParsePageSizeQuery(c, "page", "page_size", 1, 60, 120)
	if err != nil || page > 100000 {
		shared.APIError(c, 400, "分页参数无效")
		return
	}
	key := fmt.Sprintf("tag:%q:%q:%d:%d", tag, query, page, size)
	lc.graphResponse(c, key, func() (any, error) { return lc.service.BuildTagGraph(tag, query, page, size) })
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
	key := fmt.Sprintf("question:%d:%d:%t", questionID, depth, admin)
	lc.graphResponse(c, key, func() (any, error) {
		return lc.service.WithDrafts(admin).BuildQuestionGraphLimited(questionID, depth, 120, 1200)
	})
}

func (lc *LearningController) graphResponse(c *gin.Context, key string, build func() (any, error)) {
	start := time.Now()
	key = fmt.Sprintf("%d:%s", graphcache.Revision(), key)
	body, hit, err := lc.responses.Get(c.Request.Context(), key, func() ([]byte, error) {
		result, err := build()
		if err != nil {
			return nil, err
		}
		return json.Marshal(gin.H{"code": 200, "msg": "ok", "data": result})
	})
	if errors.Is(err, learningsvc.ErrLearningAmbiguousQuestion) {
		shared.APIError(c, 400, err.Error())
		return
	}
	if errors.Is(err, learningsvc.ErrLearningQuestionNotFound) {
		shared.APIError(c, 404, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, 500, "获取知识图谱失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	if hit {
		c.Header("X-Graph-Cache", "HIT")
	} else {
		c.Header("X-Graph-Cache", "MISS")
	}
	c.Header("Server-Timing", fmt.Sprintf("graph;dur=%.3f", float64(time.Since(start).Microseconds())/1000))
	c.Data(200, "application/json; charset=utf-8", body)
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
