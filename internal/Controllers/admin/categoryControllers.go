package admin

import (
	"net/http"
	"strconv"

	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
)

type CategoryController struct{}

func (con CategoryController) Index(c *gin.Context) {
	var response struct {
		Code int `json:"code"`
		Data []struct {
			Id            int    `json:"id"`
			Name          string `json:"name"`
			Slug          string `json:"slug"`
			ParentID      int    `json:"parent_id"`
			Description   string `json:"description"`
			QuestionCount int    `json:"question_count"`
		} `json:"data"`
	}

	categoryList := []models.Category{}

	err := models.DB.Preload("Question").Find(&categoryList).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分类列表失败"})
		return
	}

	for _, category := range categoryList {
		response.Data = append(response.Data, struct {
			Id            int    `json:"id"`
			Name          string `json:"name"`
			Slug          string `json:"slug"`
			ParentID      int    `json:"parent_id"`
			Description   string `json:"description"`
			QuestionCount int    `json:"question_count"`
		}{
			Id:            category.Id,
			Name:          category.Name,
			Slug:          category.Slug,
			ParentID:      category.ParentID,
			Description:   category.Description,
			QuestionCount: len(category.Question),
		})
	}

	response.Code = 200
	c.JSON(200, response)
}

func (con CategoryController) Store(c *gin.Context) {
	// 1. 绑定请求体到 Category 模型
	var category models.Category
	var request struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		ParentID    string `json:"parent_id"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	category.Name = request.Name
	category.Slug = request.Slug
	category.ParentID, _ = strconv.Atoi(request.ParentID)
	category.Description = request.Description

	// 2. 写入数据库（依赖 models.DB 已初始化）
	if err := models.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "data": nil, "msg": "创建分类失败: " + err.Error()})
		return
	}

	// 3. 返回成功响应
	c.JSON(http.StatusCreated, gin.H{"code": 200, "data": category, "msg": "分类创建成功"})
}

func (con CategoryController) Update(c *gin.Context) {
	// 1. 获取路径参数中的分类ID
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分类ID"})
		return
	}

	// 2. 绑定请求体到 Category 模型
	var request struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		ParentID    string `json:"parent_id"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var category models.Category
	category.Name = request.Name
	category.Slug = request.Slug
	category.ParentID, _ = strconv.Atoi(request.ParentID)
	category.Description = request.Description

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

// 删除分类
func (con *CategoryController) Delete(c *gin.Context) {
	// 1. 获取路径参数中的分类ID
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分类ID"})
		return
	}

	// 2. 查询分类是否存在
	var existingCategory models.Category
	if err := models.DB.First(&existingCategory, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}

	// 3. 删除分类
	if err := models.DB.Delete(&existingCategory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	// 4. 返回成功响应
	c.JSON(http.StatusOK, gin.H{"msg": "分类删除成功"})
}
