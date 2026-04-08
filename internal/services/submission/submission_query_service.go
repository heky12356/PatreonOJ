package submission

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var (
	ErrSubmissionQuestionNotFound = errors.New("question not found")
	ErrSubmissionProblemIDInvalid = errors.New("problem_id invalid")
)

type SubmissionListItem struct {
	SubmissionID string    `json:"submission_id"`
	UserID       string    `json:"user_id"`
	QuestionID   string    `json:"question_id"`
	QuestionPK   int       `json:"question_pk"`
	SubmittedAt  time.Time `json:"submitted_at"`
	Status       string    `json:"status"`
	RuntimeMs    int64     `json:"runtime_ms"`
	MemoryKB     int64     `json:"memory_kb"`
	Language     string    `json:"language"`
	CodeLength   int       `json:"code_length"`
}

type SubmissionPage struct {
	Total int64                `json:"total"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
	Pages int64                `json:"pages"`
	Items []SubmissionListItem `json:"items"`
}

type SubmissionQueryService struct {
	db *gorm.DB
}

func NewSubmissionQueryService(db *gorm.DB) *SubmissionQueryService {
	return &SubmissionQueryService{db: db}
}

func (s *SubmissionQueryService) ListProblemSubmissions(questionID int, status, language string, page, size int) (*SubmissionPage, error) {
	var question models.Question
	if err := s.db.Where("id = ?", questionID).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionQuestionNotFound
		}
		return nil, err
	}

	return s.listProblemSubmissionsByQuestion(question, status, language, page, size)
}

func (s *SubmissionQueryService) listProblemSubmissionsByQuestion(question models.Question, status, language string, page, size int) (*SubmissionPage, error) {

	countQ := s.db.Table("submissions").Joins("JOIN question ON question.id = submissions.question_id").Where("submissions.question_id = ?", question.Id)
	if status != "" {
		countQ = countQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		countQ = countQ.Where("submissions.language = ?", language)
	}

	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, err
	}

	items := make([]SubmissionListItem, 0, size)
	listQ := s.db.Table("submissions").
		Select("submissions.id AS submission_id, submissions.user_id, question.question_id, question.id AS question_pk, submissions.created_at AS submitted_at, submissions.status, submissions.runtime_ms, submissions.memory_kb, submissions.language, submissions.code_length").
		Joins("JOIN question ON question.id = submissions.question_id").
		Where("submissions.question_id = ?", question.Id)
	if status != "" {
		listQ = listQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		listQ = listQ.Where("submissions.language = ?", language)
	}
	if err := listQ.Order("submissions.created_at desc").Limit(size).Offset(size * (page - 1)).Scan(&items).Error; err != nil {
		return nil, err
	}

	pages := int64(0)
	if total > 0 {
		pages = int64(math.Ceil(float64(total) / float64(size)))
	}

	return &SubmissionPage{
		Total: total,
		Page:  page,
		Size:  size,
		Pages: pages,
		Items: items,
	}, nil
}

func (s *SubmissionQueryService) ListUserSubmissions(targetUUID, problemID, status, language string, page, size int) (*SubmissionPage, error) {
	questionID, err := s.resolveProblemID(problemID)
	if err != nil {
		return nil, err
	}

	countQ := s.db.Table("submissions").Joins("JOIN question ON question.id = submissions.question_id").Where("submissions.user_id = ?", targetUUID)
	if questionID != nil {
		countQ = countQ.Where("submissions.question_id = ?", *questionID)
	}
	if status != "" {
		countQ = countQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		countQ = countQ.Where("submissions.language = ?", language)
	}

	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, err
	}

	items := make([]SubmissionListItem, 0, size)
	listQ := s.db.Table("submissions").
		Select("submissions.id AS submission_id, submissions.user_id, question.question_id, question.id AS question_pk, submissions.created_at AS submitted_at, submissions.status, submissions.runtime_ms, submissions.memory_kb, submissions.language, submissions.code_length").
		Joins("JOIN question ON question.id = submissions.question_id").
		Where("submissions.user_id = ?", targetUUID)
	if questionID != nil {
		listQ = listQ.Where("submissions.question_id = ?", *questionID)
	}
	if status != "" {
		listQ = listQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		listQ = listQ.Where("submissions.language = ?", language)
	}
	if err := listQ.Order("submissions.created_at desc").Limit(size).Offset(size * (page - 1)).Scan(&items).Error; err != nil {
		return nil, err
	}

	pages := int64(0)
	if total > 0 {
		pages = int64(math.Ceil(float64(total) / float64(size)))
	}

	return &SubmissionPage{
		Total: total,
		Page:  page,
		Size:  size,
		Pages: pages,
		Items: items,
	}, nil
}

func (s *SubmissionQueryService) resolveProblemID(problemID string) (*int, error) {
	v := strings.TrimSpace(problemID)
	if v == "" {
		return nil, nil
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return nil, ErrSubmissionProblemIDInvalid
	}

	var q models.Question
	if err := s.db.Where("id = ?", n).First(&q).Error; err == nil {
		return &q.Id, nil
	}
	return &n, nil
}
