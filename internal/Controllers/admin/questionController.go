package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"dachuang/internal/models"
	"dachuang/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type QuestionController struct {
	db *gorm.DB
}

func NewQuestionController(db *gorm.DB) *QuestionController {
	return &QuestionController{db: db}
}

func (con QuestionController) Index(c *gin.Context) {
	questionList := []models.Question{}

	// 获取搜索内容
	q := c.DefaultQuery("q", "%")

	// 获取分页和每页大小
	pageIdx, _ := strconv.Atoi(c.DefaultQuery("pageIdx", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "3"))
	difficult := c.DefaultQuery("difficult", "")

	// 获取前端传过来的uuid
	uuid := c.DefaultQuery("uuid", "")

	if q != "%" {
		q = "%" + q + "%"
	}

	if difficult == "" {
		difficult = "%"
	}

	// 分用户组筛选
	status := "%"
	if !util.UserInstance.HasPermission(uuid, "admin") {
		status = "published"
	}

	// 先不按照分页，查找一下全部的内容，为了确定在搜索的情况下有多少个元素
	models.DB.Where("difficulty Like ? and (title Like ? or question_number Like ?) and status Like ?", difficult, q, q, status).Order("question_number ASC").Find(&questionList)
	totalCnt := len(questionList)

	// 按题目编号排序查询所有题目
	models.DB.Where("difficulty Like ? and (title Like ? or question_number Like ?) and status Like ?", difficult, q, q, status).Order("question_number ASC").Find(&questionList).Limit(pageSize).Offset(pageSize * (pageIdx - 1)).Find(&questionList)
	c.JSON(200, gin.H{
		"result":   questionList,
		"pageIdx":  pageIdx,
		"pageSize": pageSize,
		"totalCnt": totalCnt,
	})
}

func (con QuestionController) GetNewProblems(c *gin.Context) {
	questionList := []models.Question{}

	// 获取分页和每页大小
	pageIdx, _ := strconv.Atoi(c.DefaultQuery("pageIdx", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "3"))

	// 按题目编号排序查询所有题目
	models.DB.Where("status = ?", "published").Order("id DESC").Find(&questionList).Limit(pageSize).Offset(pageSize * (pageIdx - 1)).Find(&questionList)
	c.JSON(200, gin.H{
		"result":   questionList,
		"pageIdx":  pageIdx,
		"pageSize": pageSize,
	})
}

func (con QuestionController) Store(c *gin.Context) {
	var question models.Question

	// 绑定请求体到 Question 模型
	if err := c.ShouldBindJSON(&question); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	fmt.Println(question)

	// 处理题目编号逻辑
	if question.QuestionNumber == 0 {
		// 如果没有提供题目编号，自动生成
		var maxNumber int
		err := models.DB.Model(&models.Question{}).Select("COALESCE(MAX(question_number), 1000)").Scan(&maxNumber).Error
		if err != nil {
			c.JSON(500, gin.H{"error": "查询最大题目编号失败: " + err.Error()})
			return
		}
		question.QuestionNumber = maxNumber + 1
	} else {
		// 如果提供了题目编号，检查是否已存在
		var existingQuestion models.Question
		if err := models.DB.Where("question_number = ?", question.QuestionNumber).First(&existingQuestion).Error; err == nil {
			c.JSON(400, gin.H{"error": "题目编号已存在"})
			return
		}
	}

	// 设置默认值
	if question.Status == "" {
		question.Status = "draft"
	}
	if question.TimeLimit == 0 {
		question.TimeLimit = 2000 // 默认2秒
	}
	if question.MemoryLimit == 0 {
		question.MemoryLimit = 256 // 默认256MB
	}

	// 写入数据库
	if err := models.DB.Create(&question).Error; err != nil {
		c.JSON(500, gin.H{"error": "创建题目失败: " + err.Error()})
		return
	}

	// 返回成功响应
	c.JSON(201, gin.H{"data": question, "msg": "题目创建成功"})
}

func (con QuestionController) Show(c *gin.Context) {
	// 1. 获取路径参数中的题目编号
	numberStr := c.Param("number")
	if numberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少题目编号"})
		return
	}

	// 2. 将题目编号转换为整数
	questionNumber, err := strconv.Atoi(numberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	// 3. 查询题目（通过题目编号）
	var question models.Question
	if err := models.DB.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	// 4. 返回题目详情
	c.JSON(http.StatusOK, gin.H{"data": question})
}

func (con QuestionController) Update(c *gin.Context) {
	// 1. 获取路径参数中的题目编号
	numberStr := c.Param("number")
	if numberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少题目编号"})
		return
	}

	// 2. 将题目编号转换为整数
	questionNumber, err := strconv.Atoi(numberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	// 3. 绑定请求体到 Question 模型
	var question models.Question
	if err := c.ShouldBindJSON(&question); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 4. 查询题目是否存在（通过题目编号）
	var existingQuestion models.Question
	if err := models.DB.Where("question_number = ?", questionNumber).First(&existingQuestion).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	// 5. 更新题目（只更新请求中提供的字段，零值字段不更新）
	if err := models.DB.Model(&existingQuestion).Updates(question).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	// 6. 返回成功响应
	c.JSON(http.StatusOK, gin.H{"data": existingQuestion, "msg": "题目更新成功"})
}

// 删除题目
// DELETE /problem/delete/
// number: 题目编号
// uuid: 管理员uuid

type DeleteProblemRequest struct {
	Number int    `json:"number"`
	UUID   string `json:"uuid"`
}

func (con QuestionController) DeleteProblem(c *gin.Context) {
	var req DeleteProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 权限管理
	// todo

	var question models.Question
	err := con.db.Where("question_number = ?", req.Number).Find(&question).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询题目失败: " + err.Error()})
		return
	}

	// 删除题目与分类的关系
	// todo

	// 删除测试用例
	var testcases []models.TestCase
	err = con.db.Where("question_id = ?", question.Id).Find(&testcases).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询测试用例失败: " + err.Error()})
		return
	}
	for _, testcase := range testcases {
		err = con.db.Delete(&testcase).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
			return
		}
	}
	// 删除题目
	err = con.db.Delete(&question).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "题目删除成功"})
}
