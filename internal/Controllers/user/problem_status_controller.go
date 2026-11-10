package user

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/models"
	"patreonoj/internal/util"
	"strconv"
	"strings"
)

// ProblemStatuses is bound to the authenticated user, never a caller's UUID.
func (uc *UserController) ProblemStatuses(c *gin.Context) {
	value, ok := c.Get("currentUser")
	viewer, valid := value.(models.User)
	if !ok || !valid {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	raw := strings.Split(c.Query("ids"), ",")
	if len(raw) > 100 || strings.TrimSpace(c.Query("ids")) == "" {
		shared.APIError(c, 400, "provide 1 to 100 question IDs")
		return
	}
	ids := make([]int, 0, len(raw))
	for _, entry := range raw {
		id, err := strconv.Atoi(strings.TrimSpace(entry))
		if err != nil || id < 1 {
			shared.APIError(c, 400, "invalid question id")
			return
		}
		ids = append(ids, id)
	}
	visible := uc.db.Model(&models.Question{}).Where("id IN ?", ids)
	if !util.UserInstance.HasPermission(viewer, "admin") {
		visible = visible.Where("status = ?", "published")
	}
	var allowed []int
	if err := visible.Pluck("id", &allowed).Error; err != nil {
		shared.APIError(c, 500, "query failed")
		return
	}
	states := map[int]string{}
	for _, id := range allowed {
		states[id] = "unsolved"
	}
	if len(allowed) > 0 {
		var rows []struct {
			QuestionID int
			Solved     int
		}
		err := uc.db.Model(&models.Submission{}).Select("question_id, MAX(CASE WHEN verdict = 'AC' THEN 1 ELSE 0 END) AS solved").
			Where("user_id = ? AND question_id IN ?", viewer.UUID, allowed).Group("question_id").Scan(&rows).Error
		if err != nil {
			shared.APIError(c, 500, "query failed")
			return
		}
		for _, row := range rows {
			states[row.QuestionID] = "attempted"
			if row.Solved == 1 {
				states[row.QuestionID] = "solved"
			}
		}
	}
	shared.APIOK(c, 200, "ok", gin.H{"user_uuid": viewer.UUID, "statuses": states})
}
