package user

import (
	"gorm.io/gorm"
	"patreonoj/internal/models"
	submissionsvc "patreonoj/internal/services/submission"
	"patreonoj/internal/util"
	"time"
)

type HomeProblem struct {
	ID              int    `json:"id"`
	QuestionID      string `json:"question_id"`
	Title           string `json:"title"`
	Difficulty      string `json:"difficulty"`
	Attempts        int64  `json:"attempts"`
	Accepted        int    `json:"accepted"`
	LastSubmittedAt string `json:"last_submitted_at"`
}
type HomeStats struct {
	SubmissionCount int64 `json:"submission_count"`
	AttemptedCount  int64 `json:"attempted_count"`
	SolvedCount     int64 `json:"solved_count"`
	AcceptedCount   int64 `json:"accepted_count"`
}
type UserHome struct {
	ID                int                                `json:"id"`
	UUID              string                             `json:"uuid"`
	Username          string                             `json:"username"`
	Nickname          string                             `json:"nickname"`
	AvatarURL         string                             `json:"avatar_url"`
	CreatedAt         time.Time                          `json:"created_at"`
	ShowActivity      bool                               `json:"show_activity"`
	ActivityVisible   bool                               `json:"activity_visible"`
	Stats             *HomeStats                         `json:"stats,omitempty"`
	Problems          []HomeProblem                      `json:"problems,omitempty"`
	Page              int                                `json:"page"`
	Pages             int64                              `json:"pages"`
	Total             int64                              `json:"total"`
	RecentSubmissions []submissionsvc.SubmissionListItem `json:"recent_submissions,omitempty"`
}

func (s *UserService) GetHome(identifier string, viewer models.User, page, size int) (*UserHome, error) {
	target, err := s.GetByIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	admin := util.UserInstance.HasPermission(viewer, "admin")
	owner := viewer.UUID != "" && viewer.UUID == target.UUID
	home := &UserHome{ID: target.Id, UUID: target.UUID, Username: target.Username, Nickname: target.Nickname, AvatarURL: target.AvatarURL, CreatedAt: target.CreatedAt, ShowActivity: target.ShowActivity, ActivityVisible: target.ShowActivity || owner || admin, Page: page}
	if !home.ActivityVisible {
		return home, nil
	}
	base := s.db.Table("submissions AS s").Joins("JOIN question AS q ON q.id=s.question_id").
		Where("s.user_id = ? AND q.status = ?", target.UUID, "published")
	if !owner && !admin {
		base = base.Where("s.is_public = ?", true)
	}
	stats := &HomeStats{}
	if err := base.Session(&gorm.Session{}).Select("COUNT(*) AS submission_count, COUNT(DISTINCT s.question_id) AS attempted_count, COUNT(DISTINCT CASE WHEN s.verdict='AC' THEN s.question_id ELSE NULL END) AS solved_count, COALESCE(SUM(CASE WHEN s.verdict='AC' THEN 1 ELSE 0 END),0) AS accepted_count").Scan(stats).Error; err != nil {
		return nil, err
	}
	problems := make([]HomeProblem, 0, size)
	if err := base.Select("q.id,q.question_id,q.title,q.difficulty,COUNT(*) AS attempts, MAX(CASE WHEN s.verdict='AC' THEN 1 ELSE 0 END) AS accepted,MAX(s.created_at) AS last_submitted_at").
		Group("q.id,q.question_id,q.title,q.difficulty").Order("last_submitted_at DESC,q.id DESC").Limit(size).Offset((page - 1) * size).Scan(&problems).Error; err != nil {
		return nil, err
	}
	recent, err := submissionsvc.NewSubmissionQueryService(s.db).ListUserSubmissionsForViewer(target.UUID, "", "", "", 1, 10, viewer.UUID, admin)
	if err != nil {
		return nil, err
	}
	home.Stats, home.Problems, home.Total, home.Pages, home.RecentSubmissions = stats, problems, stats.AttemptedCount, (stats.AttemptedCount+int64(size)-1)/int64(size), recent.Items
	return home, nil
}
