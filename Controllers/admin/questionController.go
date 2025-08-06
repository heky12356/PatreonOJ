package admin

import (
	"github.com/gin-gonic/gin"
	"dachuang/models"
	"net/http"

)

type QuestionController struct {
    
}

func(con QuestionController) Index(c *gin.Context) {
    questionList:=[]models.Question{}

    models.DB.Find(&questionList)
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

    // 写入数据库（依赖 models.DB 已初始化）
    if err := models.DB.Create(&question).Error; err != nil {
        c.JSON(500, gin.H{"error": "创建题目失败: " + err.Error()})
        return
    }

    // 返回成功响应
    c.JSON(201, gin.H{"data": question, "msg": "题目创建成功"})
}

func (con QuestionController) Update(c *gin.Context) {
    // 1. 获取路径参数中的题目ID
    id := c.Param("id")
    if id == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "缺少题目ID"})
        return
    }

    // 2. 绑定请求体到 Question 模型
    var question models.Question
    if err := c.ShouldBindJSON(&question); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 3. 查询题目是否存在
    var existingQuestion models.Question
    if err := models.DB.First(&existingQuestion, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
        return
    }

    // 4. 更新题目（只更新请求中提供的字段，零值字段不更新）
    if err := models.DB.Model(&existingQuestion).Updates(question).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
        return
    }

    // 5. 返回成功响应
    c.JSON(http.StatusOK, gin.H{"data": existingQuestion, "msg": "题目更新成功"})
}









