package testcase

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"patreonoj/internal/config"
	"patreonoj/internal/controllers/shared"
	"patreonoj/internal/oss"
	testcasesvc "patreonoj/internal/services/testcase"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TestCaseController struct {
	service *testcasesvc.TestCaseService
}

func NewTestCaseController(db *gorm.DB, ossClient *oss.OSS) *TestCaseController {
	return &TestCaseController{
		service: testcasesvc.NewTestCaseService(db, ossClient),
	}
}

type OSSTestCaseCommitRequest struct {
	QuestionPK int    `json:"question_pk"`
	QuestionID string `json:"question_id"`
	InputKey   string `json:"input_key" binding:"required"`
	OutputKey  string `json:"output_key" binding:"required"`
	IsHidden   bool   `json:"is_hidden"`
}

func (tc *TestCaseController) Index(c *gin.Context) {
	var questionPK *int
	if v := strings.TrimSpace(c.Query("question_pk")); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil {
			shared.APIError(c, http.StatusBadRequest, "invalid question_pk")
			return
		}
		questionPK = &id
	} else if v := strings.TrimSpace(c.Query("question_id")); v != "" {
		id, err := strconv.Atoi(v)
		if err == nil {
			questionPK = &id
		}
	}

	testCases, err := tc.service.List(questionPK)
	if errors.Is(err, testcasesvc.ErrTestCaseQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "list testcases failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": testCases, "count": len(testCases)})
}

func (tc *TestCaseController) GetByQuestion(c *gin.Context) {
	questionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, "invalid question id")
		return
	}

	testCases, question, err := tc.service.GetByQuestionID(questionID)
	if errors.Is(err, testcasesvc.ErrTestCaseQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query testcases failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{
		"result":         testCases,
		"count":          len(testCases),
		"question_pk":    question.Id,
		"question_id":    question.QuestionId,
		"question_title": question.Title,
	})
}

func (tc *TestCaseController) OSSCommit(c *gin.Context) {
	var request OSSTestCaseCommitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	testCase, err := tc.service.CommitOSS(context.Background(), bucket, testcasesvc.TestCaseOSSCommitInput{
		QuestionPK: request.QuestionPK,
		QuestionID: strings.TrimSpace(request.QuestionID),
		InputKey:   request.InputKey,
		OutputKey:  request.OutputKey,
		IsHidden:   request.IsHidden,
	})
	if errors.Is(err, testcasesvc.ErrTestCaseOSSUnavailable) {
		shared.APIError(c, http.StatusInternalServerError, "oss unavailable")
		return
	}
	if errors.Is(err, testcasesvc.ErrTestCaseQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if errors.Is(err, testcasesvc.ErrTestCaseInputKeyInvalid) {
		shared.APIError(c, http.StatusBadRequest, "invalid input_key: "+err.Error())
		return
	}
	if errors.Is(err, testcasesvc.ErrTestCaseOutputKeyInvalid) {
		shared.APIError(c, http.StatusBadRequest, "invalid output_key: "+err.Error())
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "create testcase failed")
		return
	}

	shared.APIOK(c, http.StatusCreated, "create testcase success", gin.H{"result": testCase})
}

func (tc *TestCaseController) Show(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, "invalid testcase id")
		return
	}

	testCase, err := tc.service.GetByID(uint(id))
	if errors.Is(err, testcasesvc.ErrTestCaseNotFound) {
		shared.APIError(c, http.StatusNotFound, "testcase not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query testcase failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "ok", gin.H{"result": testCase})
}

func (tc *TestCaseController) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, "invalid testcase id")
		return
	}

	var request OSSTestCaseCommitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	testCase, err := tc.service.UpdateOSS(context.Background(), bucket, uint(id), testcasesvc.TestCaseOSSCommitInput{
		QuestionPK: request.QuestionPK,
		QuestionID: strings.TrimSpace(request.QuestionID),
		InputKey:   request.InputKey,
		OutputKey:  request.OutputKey,
		IsHidden:   request.IsHidden,
	})
	if errors.Is(err, testcasesvc.ErrTestCaseNotFound) {
		shared.APIError(c, http.StatusNotFound, "testcase not found")
		return
	}
	if errors.Is(err, testcasesvc.ErrTestCaseOSSUnavailable) {
		shared.APIError(c, http.StatusInternalServerError, "oss unavailable")
		return
	}
	if errors.Is(err, testcasesvc.ErrTestCaseQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if errors.Is(err, testcasesvc.ErrTestCaseInputKeyInvalid) {
		shared.APIError(c, http.StatusBadRequest, "invalid input_key: "+err.Error())
		return
	}
	if errors.Is(err, testcasesvc.ErrTestCaseOutputKeyInvalid) {
		shared.APIError(c, http.StatusBadRequest, "invalid output_key: "+err.Error())
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "update testcase failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "update testcase success", gin.H{"result": testCase})
}

func (tc *TestCaseController) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, "invalid testcase id")
		return
	}

	err = tc.service.Delete(uint(id))
	if errors.Is(err, testcasesvc.ErrTestCaseNotFound) {
		shared.APIError(c, http.StatusNotFound, "testcase not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "delete testcase failed")
		return
	}

	shared.APIOK(c, http.StatusOK, "delete testcase success", gin.H{"message": "ok"})
}

