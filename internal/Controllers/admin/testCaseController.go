package admin

import (
	"dachuang/internal/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TestCaseController 测试用例控制器
type TestCaseController struct {
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

// Index 获取测试用例列表
// GET /testcase/
// 支持查询参数: question_number (可选，按题目编号筛选)
func (tc *TestCaseController) Index(c *gin.Context) {
	var testCases []models.TestCase
	query := models.DB

	// 如果提供了question_number参数，则按题目编号筛选
	if questionNumberStr := c.Query("question_number"); questionNumberStr != "" {
		questionNumber, err := strconv.Atoi(questionNumberStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
			return
		}
		
		// 通过题目编号查找题目ID
		var question models.Question
		if err := models.DB.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
			return
		}
		
		query = query.Where("question_id = ?", question.Id)
	}

	// 查询测试用例列表，按ID排序
	if err := query.Order("id ASC").Find(&testCases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询测试用例失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	// 验证题目是否存在（通过题目编号）
	var question models.Question
	if err := models.DB.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	// 查询该题目的所有测试用例
	var testCases []models.TestCase
	if err := models.DB.Where("question_id = ?", question.Id).
		Order("id ASC").Find(&testCases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询测试用例失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":           testCases,
		"count":            len(testCases),
		"question_number":  questionNumber,
		"question_title":   question.Title,
	})
}

// Store 添加单个测试用例
// POST /testcase/
func (tc *TestCaseController) Store(c *gin.Context) {
	var request TestCaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证题目是否存在（通过题目编号）
	var question models.Question
	if err := models.DB.Where("question_number = ?", request.QuestionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	// 创建测试用例
	testCase := models.TestCase{
		QuestionID:     question.Id, // 使用题目的数据库ID
		Input:          request.Input,
		ExpectedOutput: request.ExpectedOutput,
		IsHidden:       request.IsHidden,
	}

	// 保存到数据库
	if err := models.DB.Create(&testCase).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建测试用例失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "测试用例创建成功",
		"result":  testCase,
	})
}

// BatchStore 批量添加测试用例
// POST /testcase/batch
func (tc *TestCaseController) BatchStore(c *gin.Context) {
	var request BatchTestCaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证题目是否存在（通过题目编号）
	var question models.Question
	if err := models.DB.Where("question_number = ?", request.QuestionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	// 准备批量插入的测试用例
	var testCases []models.TestCase
	for _, tc := range request.TestCases {
		testCase := models.TestCase{
			QuestionID:     question.Id, // 使用题目的数据库ID
			Input:          tc.Input,
			ExpectedOutput: tc.ExpectedOutput,
			IsHidden:       tc.IsHidden,
		}
		testCases = append(testCases, testCase)
	}

	// 批量插入到数据库
	if err := models.DB.Create(&testCases).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量创建测试用例失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "批量创建测试用例成功",
		"count":   len(testCases),
		"result":  testCases,
	})
}

// Show 获取单个测试用例详情
// GET /testcase/:id
func (tc *TestCaseController) Show(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的测试用例ID"})
		return
	}

	var testCase models.TestCase
	if err := models.DB.Where("id = ?", uint(id)).First(&testCase).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "测试用例不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": testCase,
	})
}

// Update 更新测试用例
// PUT /testcase/:id
func (tc *TestCaseController) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的测试用例ID"})
		return
	}

	// 查找测试用例
	var testCase models.TestCase
	if err := models.DB.Where("id = ?", uint(id)).First(&testCase).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "测试用例不存在"})
		return
	}

	// 绑定更新数据
	var request TestCaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果更改了题目编号，需要验证新题目是否存在
	if request.QuestionNumber != 0 {
		// 通过题目编号查找题目
		var question models.Question
		if err := models.DB.Where("question_number = ?", request.QuestionNumber).First(&question).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "目标题目不存在"})
			return
		}
		testCase.QuestionID = question.Id // 使用题目的数据库ID
	}

	// 更新测试用例
	testCase.Input = request.Input
	testCase.ExpectedOutput = request.ExpectedOutput
	testCase.IsHidden = request.IsHidden

	if err := models.DB.Save(&testCase).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新测试用例失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的测试用例ID"})
		return
	}

	// 查找测试用例
	var testCase models.TestCase
	if err := models.DB.Where("id = ?", uint(id)).First(&testCase).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "测试用例不存在"})
		return
	}

	// 检查是否是该题目的最后一个测试用例
	var count int64
	models.DB.Model(&models.TestCase{}).Where("question_id = ?", testCase.QuestionID).Count(&count)
	if count <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "不能删除题目的最后一个测试用例，每个题目至少需要一个测试用例",
		})
		return
	}

	// 删除测试用例
	if err := models.DB.Delete(&testCase).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除测试用例失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "测试用例删除成功",
	})
}