package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CategoryController struct {
	service *services.CategoryService
}

func NewCategoryController(db *gorm.DB) *CategoryController {
	return &CategoryController{
		service: services.NewCategoryService(db),
	}
}

func (con *CategoryController) Index(c *gin.Context) {
	data, err := con.service.List()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "获取分类列表失败")
		return
	}
	apiOK(c, http.StatusOK, "ok", gin.H{"data": data})
}

func (con *CategoryController) Store(c *gin.Context) {
	var request struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		ParentID    string `json:"parent_id"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	parentID := 0
	if strings.TrimSpace(request.ParentID) != "" {
		v, err := strconv.Atoi(strings.TrimSpace(request.ParentID))
		if err != nil {
			apiError(c, http.StatusBadRequest, "parent_id 必须是整数")
			return
		}
		parentID = v
	}

	category, err := con.service.Create(services.CategoryCreateInput{
		Name:        request.Name,
		Slug:        request.Slug,
		ParentID:    parentID,
		Description: request.Description,
	})
	if err != nil {
		apiError(c, http.StatusInternalServerError, "创建分类失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusCreated, "分类创建成功", gin.H{"data": category})
}

func (con *CategoryController) Update(c *gin.Context) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		apiError(c, http.StatusBadRequest, "分类ID无效")
		return
	}

	var request struct {
		Name        *string `json:"name"`
		Slug        *string `json:"slug"`
		ParentID    *string `json:"parent_id"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	input := services.CategoryUpdateInput{
		Name:        request.Name,
		Slug:        request.Slug,
		Description: request.Description,
	}
	if request.ParentID != nil && strings.TrimSpace(*request.ParentID) != "" {
		parentID, err := strconv.Atoi(strings.TrimSpace(*request.ParentID))
		if err != nil {
			apiError(c, http.StatusBadRequest, "parent_id 必须是整数")
			return
		}
		input.ParentID = &parentID
	}

	category, err := con.service.Update(id, input)
	if errors.Is(err, services.ErrCategoryNotFound) {
		apiError(c, http.StatusNotFound, "分类不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusOK, "分类更新成功", gin.H{"data": category})
}

func (con *CategoryController) Delete(c *gin.Context) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		apiError(c, http.StatusBadRequest, "分类ID无效")
		return
	}

	err = con.service.Delete(id)
	if errors.Is(err, services.ErrCategoryNotFound) {
		apiError(c, http.StatusNotFound, "分类不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}

	apiOK(c, http.StatusOK, "分类删除成功", gin.H{})
}
