package admin

import (
	"net/http"

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
	userID := c.Param("uuid")

	var numbers []int
	if err := uc.db.Table("user_solved_question usq").
		Joins("JOIN question q ON q.id = usq.question_id").
		Where("usq.user_uuid = ?", userID).
		Order("usq.solved_at DESC").
		Pluck("q.question_number", &numbers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户解题列表失败"})
		return
	}
	c.JSON(http.StatusOK, numbers)
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

	var cnt int64
	if err := uc.db.Table("user_solved_question usq").
		Joins("JOIN question q ON q.id = usq.question_id").
		Where("usq.user_uuid = ? AND q.question_number = ?", UUID, questionNumber).
		Count(&cnt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户解题详情失败"})
		return
	}
	solved := cnt > 0
	c.JSON(http.StatusOK, gin.H{"solved": solved})
}
