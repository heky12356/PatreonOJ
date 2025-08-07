package admin

import (
	"github.com/gin-gonic/gin"
	"dachuang/internal/models"
	"net/http"
	"strconv"
)

type QuestionController struct {
    
}

func(con QuestionController) Index(c *gin.Context) {
    questionList:=[]models.Question{}

    // 按题目编号排序查询所有题目
    models.DB.Order("question_number ASC").Find(&questionList)
	c.JSON(200, gin.H{
	 "result":questionList ,
	})
}
func (con QuestionController) Store(c *gin.Context) {
    var question models.Question
    
    // 绑定请求体到 Question 模型
    if err := c.ShouldBindJSON(&question); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

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









