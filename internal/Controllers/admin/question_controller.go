package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"dachuang/internal/models"
	"dachuang/internal/services"
	"dachuang/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type QuestionController struct {
	service *services.QuestionService
}

func NewQuestionController(db *gorm.DB) *QuestionController {
	return &QuestionController{
		service: services.NewQuestionService(db),
	}
}

func (con *QuestionController) Index(c *gin.Context) {
	pageIdx, pageSize, err := parsePageSizeQuery(c, "pageIdx", "pageSize", 1, 3, 100)
	if err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	q := c.DefaultQuery("q", "%")
	difficult := c.DefaultQuery("difficult", "")
	uuid := c.DefaultQuery("uuid", "")
	isAdmin := util.UserInstance.HasPermission(uuid, "admin")

	result, err := con.service.List(q, difficult, isAdmin, pageIdx, pageSize)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询题目失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"result":   result.Result,
		"pageIdx":  result.PageIdx,
		"pageSize": result.PageSize,
		"totalCnt": result.TotalCnt,
	})
}

func (con *QuestionController) GetNewProblems(c *gin.Context) {
	pageIdx, pageSize, err := parsePageSizeQuery(c, "pageIdx", "pageSize", 1, 3, 100)
	if err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	items, err := con.service.ListNewestPublished(pageIdx, pageSize)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询题目失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"result":   items,
		"pageIdx":  pageIdx,
		"pageSize": pageSize,
	})
}

func (con *QuestionController) Store(c *gin.Context) {
	var request struct {
		QuestionNumber int    `json:"question_number"`
		QuestionID     string `json:"question_id"`
		Title          string `json:"title"`
		Content        string `json:"content"`
		Difficulty     string `json:"difficulty"`
		DataRange      string `json:"data_range"`
		TimeLimit      int    `json:"time_limit"`
		MemoryLimit    int    `json:"memory_limit"`
		Tags           string `json:"tags"`
		CategoryID     string `json:"category_id"`
		Status         string `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	categoryID := 0
	if strings.TrimSpace(request.CategoryID) != "" {
		v, err := strconv.Atoi(strings.TrimSpace(request.CategoryID))
		if err != nil {
			apiError(c, http.StatusBadRequest, "category_id 必须是整数")
			return
		}
		categoryID = v
	}

	question, err := con.service.Create(services.QuestionCreateInput{
		QuestionNumber: request.QuestionNumber,
		QuestionID:     request.QuestionID,
		Title:          request.Title,
		Content:        request.Content,
		Difficulty:     request.Difficulty,
		DataRange:      request.DataRange,
		TimeLimit:      request.TimeLimit,
		MemoryLimit:    request.MemoryLimit,
		Tags:           request.Tags,
		CategoryID:     categoryID,
		Status:         request.Status,
	})
	if errors.Is(err, services.ErrQuestionNumberExists) {
		apiError(c, http.StatusBadRequest, "题目编号已存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "创建题目失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusCreated, "题目创建成功", gin.H{"data": question})
}

func (con *QuestionController) Show(c *gin.Context) {
	numberStr := c.Param("number")
	if numberStr == "" {
		apiError(c, http.StatusBadRequest, "缺少题目编号")
		return
	}
	questionNumber, err := strconv.Atoi(numberStr)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效的题目编号")
		return
	}

	question, err := con.service.GetByNumber(questionNumber)
	if errors.Is(err, services.ErrQuestionRecordNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询题目失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{"data": question})
}

func (con *QuestionController) ShowByQuestionID(c *gin.Context) {
	questionID := strings.TrimSpace(c.Param("question_id"))
	if questionID == "" {
		apiError(c, http.StatusBadRequest, "缺少question_id")
		return
	}

	question, err := con.service.GetByQuestionID(questionID)
	if errors.Is(err, services.ErrQuestionRecordNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询题目失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{"data": question})
}

func (con *QuestionController) Update(c *gin.Context) {
	numberStr := c.Param("number")
	if numberStr == "" {
		apiError(c, http.StatusBadRequest, "缺少题目编号")
		return
	}
	questionNumber, err := strconv.Atoi(numberStr)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效的题目编号")
		return
	}

	var payload models.Question
	if err := c.ShouldBindJSON(&payload); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	question, err := con.service.UpdateByNumber(questionNumber, payload)
	if errors.Is(err, services.ErrQuestionRecordNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusOK, "题目更新成功", gin.H{"data": question})
}

type DeleteProblemRequest struct {
	Number int    `json:"number"`
	UUID   string `json:"uuid"`
}

func (con *QuestionController) DeleteProblem(c *gin.Context) {
	var req DeleteProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := con.service.DeleteByNumber(req.Number); errors.Is(err, services.ErrQuestionRecordNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	} else if err != nil {
		apiError(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusOK, "题目删除成功", gin.H{})
}
