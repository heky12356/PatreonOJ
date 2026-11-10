package user

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"patreonoj/internal/models"
	"patreonoj/internal/util"
	"strings"
)

type UserSolveController struct{ db *gorm.DB }

func NewUserSolveController(db *gorm.DB) *UserSolveController { return &UserSolveController{db: db} }

func (uc *UserSolveController) query(c *gin.Context, uuid string) (*gorm.DB, bool) {
	var target models.User
	if err := uc.db.Where("uuid = ?", uuid).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "user not found"})
		} else {
			c.JSON(500, gin.H{"error": "query failed"})
		}
		return nil, false
	}
	viewer := models.User{}
	if value, ok := c.Get("currentUser"); ok {
		viewer, _ = value.(models.User)
	}
	privileged := viewer.UUID == target.UUID || util.UserInstance.HasPermission(viewer, "admin")
	if !privileged && !target.ShowActivity {
		c.JSON(403, gin.H{"error": "activity is private"})
		return nil, false
	}
	query := uc.db.Table("submissions s").Joins("JOIN question q ON q.id=s.question_id").Where("s.user_id = ? AND s.verdict = ?", target.UUID, "AC")
	if !privileged {
		query = query.Where("q.status = ? AND s.is_public = ?", "published", true)
	}
	return query, true
}
func (uc *UserSolveController) Index(c *gin.Context) {
	query, ok := uc.query(c, c.Param("uuid"))
	if !ok {
		return
	}
	numbers := []int{}
	if err := query.Distinct("s.question_id").Pluck("s.question_id", &numbers).Error; err != nil {
		c.JSON(500, gin.H{"error": "query failed"})
		return
	}
	c.JSON(http.StatusOK, numbers)
}
func (uc *UserSolveController) Show(c *gin.Context) {
	uuid, id := strings.TrimSpace(c.Query("uuid")), strings.TrimSpace(c.Query("id"))
	if id == "" {
		id = strings.TrimSpace(c.Query("question_number"))
	}
	if uuid == "" || id == "" {
		c.JSON(400, gin.H{"error": "invalid user solve query"})
		return
	}
	query, ok := uc.query(c, uuid)
	if !ok {
		return
	}
	var count int64
	if err := query.Where("q.id = ?", id).Count(&count).Error; err != nil {
		c.JSON(500, gin.H{"error": "query failed"})
		return
	}
	c.JSON(200, gin.H{"solved": count > 0})
}
