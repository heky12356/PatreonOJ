package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserSolveController struct {
	db *gorm.DB
}

func NewUserSolveController(db *gorm.DB) *UserSolveController {
	return &UserSolveController{
		db: db,
	}
}

// Index 获取用户解题列表
// GET /admin/solves/:uuid
func (uc *UserSolveController) Index(c *gin.Context) {
	var userSolve models.UserSolve
	userID := c.Param("uuid")
	if err := uc.db.Where("uuid = ?", userID).Find(&userSolve).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, []string{})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户解题列表失败"})
		return
	}
	// 转换 ProblemIDs 为字符串数组
	problemIDs := strings.Split(userSolve.ProblemIDs, ",")

	c.JSON(http.StatusOK, problemIDs)
}

// Show 获取用户解题详情
// GET /admin/solve/:id
func (uc *UserSolveController) Show(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户解题ID"})
		return
	}

	var userSolve models.UserSolve
	if err := models.DB.Where("id = ?", id).First(&userSolve).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户解题不存在"})
		return
	}

	// 转换 ProblemIDs 为字符串数组
	var problemIDs []string
	if err := json.Unmarshal([]byte(userSolve.ProblemIDs), &problemIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "处理用户解题数据失败"})
		return
	}

	// 转换 UUID 为字符串
	userID := userSolve.UUID

	// 构建响应数据
	response := gin.H{
		"id":          userSolve.ID,
		"user_id":     userID,
		"problem_ids": problemIDs,
		"created_at":  userSolve.CreatedAt,
		"updated_at":  userSolve.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}
