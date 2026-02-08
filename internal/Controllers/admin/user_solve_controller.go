package admin

import (
	"net/http"
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
	result := uc.db.Where("uuid = ?", userID).First(&userSolve)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户解题列表失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusOK, []string{})
		return
	}
	// 转换 ProblemIDs 为字符串数组
	problemIDs := strings.Split(userSolve.ProblemIDs, ",")

	c.JSON(http.StatusOK, problemIDs)
}

// Show 获取用户解题详情
// GET /admin/solve?uuid=&question_number=
func (uc *UserSolveController) Show(c *gin.Context) {
	UUID := c.Query("uuid")
	questionNumber := c.Query("question_number")
	if UUID == "" || questionNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户解题ID"})
		return
	}

	var userSolve models.UserSolve
	if err := uc.db.Where("uuid = ?", UUID).First(&userSolve).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户解题不存在"})
		return
	}

	ProblemIDs := strings.Split(userSolve.ProblemIDs, ",")
	// 检查问题编号是否在用户已解决的问题列表中
	found := false
	for _, id := range ProblemIDs {
		if id == questionNumber {
			found = true
			break
		}
	}
	if found {
		c.JSON(http.StatusOK, gin.H{"solved": true})
	} else {
		c.JSON(http.StatusOK, gin.H{"solved": false})
	}
}
