package admin

import (
	"github.com/gin-gonic/gin"
	"dachuang/models"
	  "net/http"
   
)

type CategoryController struct {
    
}

func(con CategoryController) Index(c *gin.Context) {
   //categoryList := []models.Category{}
	//models.DB.Find(&categoryList)
	//c.JSON(200, gin.H{
	// "result":categoryList ,
	//})
	 categoryList := []models.Category{}

    models.DB.Preload("Question").Find(&categoryList)
	c.JSON(200, gin.H{
	 "result":categoryList ,
	})
}
func (con CategoryController) Store(c *gin.Context) {
    // 1. 绑定请求体到 Category 模型
    var category models.Category
    if err := c.ShouldBindJSON(&category); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 2. 写入数据库（依赖 models.DB 已初始化）
    if err := models.DB.Create(&category).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "创建分类失败: " + err.Error()})
        return
    }

    // 3. 返回成功响应
    c.JSON(http.StatusCreated, gin.H{"data": category, "msg": "分类创建成功"})
}


func (con CategoryController) Update(c *gin.Context) {
    // 1. 获取路径参数中的分类ID
    id := c.Param("id")
    if id == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分类ID"})
        return
    }

    // 2. 绑定请求体到 Category 模型
    var category models.Category
    if err := c.ShouldBindJSON(&category); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 3. 查询分类是否存在
    var existingCategory models.Category
    if err := models.DB.First(&existingCategory, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
        return
    }

    // 4. 更新分类（只更新请求中提供的字段，零值字段不更新）
    if err := models.DB.Model(&existingCategory).Updates(category).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
        return
    }

    // 5. 返回成功响应
    c.JSON(http.StatusOK, gin.H{"data": existingCategory, "msg": "分类更新成功"})
}