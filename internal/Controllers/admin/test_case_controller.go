package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"dachuang/internal/config"
	"dachuang/internal/oss"
	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TestCaseController 测试用例控制器
type TestCaseController struct {
	service *services.TestCaseService
}

func NewTestCaseController(db *gorm.DB, ossClient *oss.OSS) *TestCaseController {
	return &TestCaseController{
		service: services.NewTestCaseService(db, ossClient),
	}
}

// TestCaseRequest 测试用例请求结构体
type TestCaseRequest struct {
	QuestionNumber int    `json:"question_number" binding:"required"` // 题目编号
	Input          string `json:"input" binding:"required"`           // 输入数据
	ExpectedOutput string `json:"expected_output" binding:"required"` // 期望输出
	IsHidden       bool   `json:"is_hidden"`                          // 是否隐藏测试用例
}

// BatchTestCaseRequest 批量添加测试用例请求结构体
type BatchTestCaseRequest struct {
	QuestionNumber int `json:"question_number" binding:"required"` // 题目编号
	TestCases      []struct {
		Input          string `json:"input" binding:"required"`           // 输入数据
		ExpectedOutput string `json:"expected_output" binding:"required"` // 期望输出
		IsHidden       bool   `json:"is_hidden"`                          // 是否隐藏测试用例
	} `json:"test_cases" binding:"required,min=1"` // 测试用例列表，至少包含一个
}

type OSSTestCaseCommitRequest struct {
	QuestionNumber int    `json:"question_number" binding:"required"`
	InputKey       string `json:"input_key" binding:"required"`
	OutputKey      string `json:"output_key" binding:"required"`
	IsHidden       bool   `json:"is_hidden"`
}

// Index 获取测试用例列表
// GET /testcase/
// 支持查询参数: question_number (可选，按题目编号筛选)
func (tc *TestCaseController) Index(c *gin.Context) {
	var questionNumber *int
	if questionNumberStr := c.Query("question_number"); questionNumberStr != "" {
		num, err := strconv.Atoi(questionNumberStr)
		if err != nil {
			apiError(c, http.StatusBadRequest, "无效的题目编号")
			return
		}
		questionNumber = &num
	}

	testCases, err := tc.service.List(questionNumber)
	if errors.Is(err, services.ErrTestCaseQuestionNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询测试用例失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"result": testCases,
		"count":  len(testCases),
	})
}

// GetByQuestion 根据题目编号获取测试用例
// GET /testcase/question/:number
func (tc *TestCaseController) GetByQuestion(c *gin.Context) {
	questionNumberStr := c.Param("number")
	questionNumber, err := strconv.Atoi(questionNumberStr)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效的题目编号")
		return
	}

	testCases, question, err := tc.service.GetByQuestionNumber(questionNumber)
	if errors.Is(err, services.ErrTestCaseQuestionNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询测试用例失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"result":          testCases,
		"count":           len(testCases),
		"question_number": questionNumber,
		"question_title":  question.Title,
	})
}

// Store 添加单个测试用例
// POST /testcase/
func (tc *TestCaseController) Store(c *gin.Context) {
	var request TestCaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	testCase, err := tc.service.Create(services.TestCaseCreateInput{
		QuestionNumber: request.QuestionNumber,
		Input:          request.Input,
		ExpectedOutput: request.ExpectedOutput,
		IsHidden:       request.IsHidden,
	})
	if errors.Is(err, services.ErrTestCaseQuestionNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "创建测试用例失败")
		return
	}

	apiOK(c, http.StatusCreated, "测试用例创建成功", gin.H{
		"message": "测试用例创建成功",
		"result":  testCase,
	})
}

// BatchStore 批量添加测试用例
// POST /testcase/batch
func (tc *TestCaseController) BatchStore(c *gin.Context) {
	var request BatchTestCaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	items := make([]services.TestCaseBatchItem, 0, len(request.TestCases))
	for _, item := range request.TestCases {
		items = append(items, services.TestCaseBatchItem{
			Input:          item.Input,
			ExpectedOutput: item.ExpectedOutput,
			IsHidden:       item.IsHidden,
		})
	}

	testCases, err := tc.service.BatchCreate(request.QuestionNumber, items)
	if errors.Is(err, services.ErrTestCaseQuestionNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "批量创建测试用例失败")
		return
	}

	apiOK(c, http.StatusCreated, "批量创建测试用例成功", gin.H{
		"message": "批量创建测试用例成功",
		"count":   len(testCases),
		"result":  testCases,
	})
}

func (tc *TestCaseController) OSSCommit(c *gin.Context) {
	var request OSSTestCaseCommitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	testCase, err := tc.service.CommitOSS(context.Background(), bucket, services.TestCaseOSSCommitInput{
		QuestionNumber: request.QuestionNumber,
		InputKey:       request.InputKey,
		OutputKey:      request.OutputKey,
		IsHidden:       request.IsHidden,
	})
	if errors.Is(err, services.ErrTestCaseOSSUnavailable) {
		apiError(c, http.StatusInternalServerError, "OSS 未初始化")
		return
	}
	if errors.Is(err, services.ErrTestCaseQuestionNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if errors.Is(err, services.ErrTestCaseInputKeyInvalid) {
		apiError(c, http.StatusBadRequest, "input_key 不存在或不可访问: "+err.Error())
		return
	}
	if errors.Is(err, services.ErrTestCaseOutputKeyInvalid) {
		apiError(c, http.StatusBadRequest, "output_key 不存在或不可访问: "+err.Error())
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "创建测试用例失败")
		return
	}

	apiOK(c, http.StatusCreated, "测试用例创建成功", gin.H{
		"message": "测试用例创建成功",
		"result":  testCase,
	})
}

// Show 获取单个测试用例详情
// GET /testcase/:id
func (tc *TestCaseController) Show(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效的测试用例ID")
		return
	}

	testCase, err := tc.service.GetByID(uint(id))
	if errors.Is(err, services.ErrTestCaseNotFound) {
		apiError(c, http.StatusNotFound, "测试用例不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询测试用例失败")
		return
	}

	apiOK(c, http.StatusOK, "ok", gin.H{
		"result": testCase,
	})
}

// Update 更新测试用例
// PUT /testcase/:id
func (tc *TestCaseController) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效的测试用例ID")
		return
	}

	// 绑定更新数据
	var request TestCaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	testCase, err := tc.service.Update(uint(id), services.TestCaseCreateInput{
		QuestionNumber: request.QuestionNumber,
		Input:          request.Input,
		ExpectedOutput: request.ExpectedOutput,
		IsHidden:       request.IsHidden,
	})
	if errors.Is(err, services.ErrTestCaseNotFound) {
		apiError(c, http.StatusNotFound, "测试用例不存在")
		return
	}
	if errors.Is(err, services.ErrTestCaseQuestionNotFound) {
		apiError(c, http.StatusNotFound, "目标题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "更新测试用例失败")
		return
	}

	apiOK(c, http.StatusOK, "测试用例更新成功", gin.H{
		"message": "测试用例更新成功",
		"result":  testCase,
	})
}

// Delete 删除测试用例
// DELETE /testcase/:id
func (tc *TestCaseController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiError(c, http.StatusBadRequest, "无效的测试用例ID")
		return
	}

	err = tc.service.Delete(uint(id))
	if errors.Is(err, services.ErrTestCaseNotFound) {
		apiError(c, http.StatusNotFound, "测试用例不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "删除测试用例失败")
		return
	}

	apiOK(c, http.StatusOK, "测试用例删除成功", gin.H{"message": "测试用例删除成功"})
}
