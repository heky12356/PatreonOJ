package user

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserSolveController struct {
	db *gorm.DB
}

func NewUserSolveController(db *gorm.DB) *UserSolveController {
	return &UserSolveController{db: db}
}

// GET /user/solves/:uuid
func (uc *UserSolveController) Index(c *gin.Context) {
	userID := c.Param("uuid")

	var numbers []int
	if err := uc.db.Table("user_solved_question usq").
		Joins("JOIN question q ON q.id = usq.question_id").
		Where("usq.user_uuid = ?", userID).
		Order("usq.solved_at DESC").
		Pluck("q.id", &numbers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query user solved list failed"})
		return
	}
	c.JSON(http.StatusOK, numbers)
}

// GET /user/solve?uuid=&id=
func (uc *UserSolveController) Show(c *gin.Context) {
	uuid := strings.TrimSpace(c.Query("uuid"))
	questionID := strings.TrimSpace(c.Query("id"))
	if questionID == "" {
		// backward compatibility
		questionID = strings.TrimSpace(c.Query("question_number"))
	}
	if uuid == "" || questionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user solve query"})
		return
	}

	var cnt int64
	if err := uc.db.Table("user_solved_question usq").
		Joins("JOIN question q ON q.id = usq.question_id").
		Where("usq.user_uuid = ? AND q.id = ?", uuid, questionID).
		Count(&cnt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query user solved detail failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"solved": cnt > 0})
}
