package submission

import (
	"errors"
	"gorm.io/gorm"
	"patreonoj/internal/models"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSubmissionQuestionNotFound = errors.New("question not found")
	ErrSubmissionProblemIDInvalid = errors.New("problem_id invalid")
)

type SubmissionListItem struct {
	SubmissionID  string    `json:"submission_id"`
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	QuestionID    string    `json:"question_id"`
	QuestionTitle string    `json:"question_title"`
	QuestionPK    int       `json:"question_pk"`
	SubmittedAt   time.Time `json:"submitted_at"`
	Status        string    `json:"status"`
	Verdict       string    `json:"verdict"`
	ErrorCode     string    `json:"-"`
	RuntimeMs     int64     `json:"runtime_ms"`
	MemoryKB      int64     `json:"memory_kb"`
	Language      string    `json:"language"`
	CodeLength    int       `json:"code_length"`
}
type SubmissionPage struct {
	Total int64                `json:"total"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
	Pages int64                `json:"pages"`
	Items []SubmissionListItem `json:"items"`
}
type SubmissionQueryService struct{ db *gorm.DB }

func NewSubmissionQueryService(db *gorm.DB) *SubmissionQueryService {
	return &SubmissionQueryService{db: db}
}

func (s *SubmissionQueryService) ListProblemSubmissions(questionID int, status, language string, page, size int) (*SubmissionPage, error) {
	return s.ListProblemSubmissionsForViewer(questionID, status, language, page, size, "", false)
}
func (s *SubmissionQueryService) ListProblemSubmissionsForViewer(questionID int, status, language string, page, size int, viewerUUID string, isAdmin bool) (*SubmissionPage, error) {
	var q models.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionQuestionNotFound
		}
		return nil, err
	}
	if !isAdmin && q.Status != "published" {
		return nil, ErrSubmissionQuestionNotFound
	}
	return s.list(s.base(viewerUUID, isAdmin).Where("submissions.question_id = ?", q.Id), status, language, page, size)
}
func (s *SubmissionQueryService) ListUserSubmissions(targetUUID, problemID, status, language string, page, size int) (*SubmissionPage, error) {
	return s.ListUserSubmissionsForViewer(targetUUID, problemID, status, language, page, size, targetUUID, false)
}
func (s *SubmissionQueryService) ListUserSubmissionsForViewer(targetUUID, problemID, status, language string, page, size int, viewerUUID string, isAdmin bool) (*SubmissionPage, error) {
	id, err := s.resolveProblemID(problemID)
	if err != nil {
		return nil, err
	}
	query := s.base(viewerUUID, isAdmin).Where("submissions.user_id = ?", targetUUID)
	if id != nil {
		query = query.Where("submissions.question_id = ?", *id)
	}
	if viewerUUID != targetUUID && !isAdmin {
		query = query.Where("question.status = ?", "published")
	}
	return s.list(query, status, language, page, size)
}
func (s *SubmissionQueryService) base(viewerUUID string, isAdmin bool) *gorm.DB {
	query := s.db.Table("submissions").Joins("JOIN question ON question.id = submissions.question_id").
		Joins("LEFT JOIN user AS author ON author.uuid = submissions.user_id")
	if !isAdmin {
		query = query.Where("(submissions.is_public = ? AND (author.show_activity = ? OR author.uuid IS NULL)) OR submissions.user_id = ?", true, true, viewerUUID)
	}
	return query
}
func (s *SubmissionQueryService) list(query *gorm.DB, status, language string, page, size int) (*SubmissionPage, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	switch value := strings.ToUpper(strings.TrimSpace(status)); value {
	case "":
	case "PENDING":
		query = query.Where("submissions.status = ?", "pending")
	case "JUDGING", "PROCESSING":
		query = query.Where("submissions.status = ?", "processing")
	case "AC", "WA", "RE", "TLE", "MLE", "OLE", "CE", "SE":
		query = query.Where("submissions.verdict = ?", value)
	default:
		query = query.Where("submissions.status = ?", strings.ToLower(value))
	}
	if language != "" {
		query = query.Where("submissions.language = ?", language)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}
	items := make([]SubmissionListItem, 0, size)
	err := query.Select("submissions.id AS submission_id, submissions.user_id, COALESCE(author.username, '') AS username, question.question_id, question.title AS question_title, question.id AS question_pk, submissions.created_at AS submitted_at, submissions.status, submissions.verdict, submissions.error_code, submissions.runtime_ms, submissions.memory_kb, submissions.language, submissions.code_length").
		Order("submissions.created_at DESC, submissions.id DESC").Limit(size).Offset(size * (page - 1)).Scan(&items).Error
	if err != nil {
		return nil, err
	}
	for i := range items {
		row := &items[i]
		row.Verdict = (models.Submission{Status: row.Status, Verdict: row.Verdict, ErrorCode: row.ErrorCode}).ResultVerdict()
	}
	return &SubmissionPage{Total: total, Page: page, Size: size, Pages: (total + int64(size) - 1) / int64(size), Items: items}, nil
}
func (s *SubmissionQueryService) resolveProblemID(problemID string) (*int, error) {
	if strings.TrimSpace(problemID) == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(problemID))
	if err != nil || n <= 0 {
		return nil, ErrSubmissionProblemIDInvalid
	}
	return &n, nil
}
