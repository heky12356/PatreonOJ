package question

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"patreonoj/internal/controllers/shared"
	"patreonoj/internal/models"
	questionsvc "patreonoj/internal/services/question"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type QuestionController struct {
	service     *questionsvc.QuestionService
	edgeBuilder questionsvc.QuestionEdgeBuilder
}

func NewQuestionController(db *gorm.DB, edgeBuilder questionsvc.QuestionEdgeBuilder) *QuestionController {
	return &QuestionController{
		service:     questionsvc.NewQuestionService(db),
		edgeBuilder: edgeBuilder,
	}
}

func (con *QuestionController) Index(c *gin.Context) {
	pageIdx, pageSize, err := shared.ParsePageSizeQuery(c, "pageIdx", "pageSize", 1, 3, 100)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	q := c.DefaultQuery("q", "%")
	difficult := c.DefaultQuery("difficult", "")
	isAdmin := false
	user, ok := c.Get("currentUser")
	if ok {
		isAdmin = util.UserInstance.HasPermission(user.(models.User), "admin")
	}

	result, err := con.service.List(q, difficult, isAdmin, pageIdx, pageSize)
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query question failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{
		"result":   result.Result,
		"pageIdx":  result.PageIdx,
		"pageSize": result.PageSize,
		"totalCnt": result.TotalCnt,
	})
}

func (con *QuestionController) GetNewProblems(c *gin.Context) {
	pageIdx, pageSize, err := shared.ParsePageSizeQuery(c, "pageIdx", "pageSize", 1, 3, 100)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	items, err := con.service.ListNewestPublished(pageIdx, pageSize)
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query question failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{
		"result":   items,
		"pageIdx":  pageIdx,
		"pageSize": pageSize,
	})
}

// 新增题面
func (con *QuestionController) Store(c *gin.Context) {
	var request struct {
		QuestionID  string `json:"question_id"`
		Title       string `json:"title"`
		Content     string `json:"content"`
		Difficulty  string `json:"difficulty"`
		TimeLimit   int    `json:"time_limit"`
		MemoryLimit int    `json:"memory_limit"`
		Tags        string `json:"tags"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	question, err := con.service.Create(questionsvc.QuestionCreateInput{
		QuestionID:  strings.TrimSpace(request.QuestionID),
		Title:       request.Title,
		Content:     request.Content,
		Difficulty:  request.Difficulty,
		TimeLimit:   request.TimeLimit,
		MemoryLimit: request.MemoryLimit,
		Tags:        request.Tags,
		Status:      request.Status,
	})
	if errors.Is(err, questionsvc.ErrQuestionIDRequired) {
		shared.APIError(c, http.StatusBadRequest, "question_id is required")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "create question failed: "+err.Error())
		return
	}

	if con.edgeBuilder != nil {
		con.edgeBuilder.Enqueue(question.Id)
	}

	shared.APIOK(c, http.StatusCreated, "create question success", gin.H{"data": question})
}

func (con *QuestionController) Show(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		shared.APIError(c, http.StatusBadRequest, "missing question id")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, "invalid question id")
		return
	}

	question, err := con.service.GetByID(id)

	user, ok := c.Get("currentUser")
	isAdmin := false
	if ok {
		isAdmin = util.UserInstance.HasPermission(user.(models.User), "admin")
	}
	if !isAdmin && question.Status != "published" {
		shared.APIError(c, http.StatusForbidden, "没有权限")
		return
	}

	if errors.Is(err, questionsvc.ErrQuestionRecordNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query question failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{"data": question})
}

// 更新题面
func (con *QuestionController) Update(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		shared.APIError(c, http.StatusBadRequest, "missing question id")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, "invalid question id")
		return
	}

	var payload models.Question
	if err := c.ShouldBindJSON(&payload); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	question, err := con.service.UpdateByID(id, payload)
	if errors.Is(err, questionsvc.ErrQuestionRecordNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "update question failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "update question success", gin.H{"data": question})
}

type DeleteProblemRequest struct {
	ID   int    `json:"id"`
	UUID string `json:"uuid"`
}

// 删除题目
func (con *QuestionController) DeleteProblem(c *gin.Context) {
	var req DeleteProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	err := con.service.DeleteByID(req.ID)
	if errors.Is(err, questionsvc.ErrQuestionRecordNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "delete question failed: "+err.Error())
		return
	}

	shared.APIOK(c, http.StatusOK, "delete question success", gin.H{})
}
