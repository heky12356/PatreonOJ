package question

import (
	"errors"
	"strconv"
	"strings"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var (
	ErrQuestionRecordNotFound = errors.New("question not found")
	ErrQuestionIDRequired     = errors.New("question_id is required")
)

type QuestionListResult struct {
	Result   []models.Question `json:"result"`
	PageIdx  int               `json:"pageIdx"`
	PageSize int               `json:"pageSize"`
	TotalCnt int64             `json:"totalCnt"`
}

type QuestionCreateInput struct {
	QuestionID  string
	Title       string
	Content     string
	Difficulty  string
	TimeLimit   int
	MemoryLimit int
	Tags        string
	Status      string
}

type QuestionService struct {
	db *gorm.DB
}

func NewQuestionService(db *gorm.DB) *QuestionService {
	return &QuestionService{db: db}
}

func (s *QuestionService) List(q, difficult string, isAdmin bool, pageIdx, pageSize int) (*QuestionListResult, error) {
	query := strings.TrimSpace(q)
	if query == "" {
		query = "%"
	}
	queryNumber := 0
	queryHasNumber := false
	if query != "%" {
		if n, err := strconv.Atoi(query); err == nil {
			queryHasNumber = true
			queryNumber = n
		}
	}
	if query != "%" {
		query = "%" + query + "%"
	}

	diff := strings.TrimSpace(difficult)
	if diff == "" {
		diff = "%"
	}

	status := "%"
	if !isAdmin {
		status = "published"
	}

	base := s.db.Model(&models.Question{}).Where("difficulty LIKE ? AND status LIKE ?", diff, status)
	if query != "%" {
		if queryHasNumber {
			base = base.Where(
				"(title LIKE ? OR question_id LIKE ? OR tags LIKE ? OR content LIKE ? OR id = ?)",
				query,
				query,
				query,
				query,
				queryNumber,
			)
		} else {
			base = base.Where(
				"(title LIKE ? OR question_id LIKE ? OR tags LIKE ? OR content LIKE ?)",
				query,
				query,
				query,
				query,
			)
		}
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	items := make([]models.Question, 0, pageSize)
	if err := base.Order("id ASC").Limit(pageSize).Offset(pageSize * (pageIdx - 1)).Find(&items).Error; err != nil {
		return nil, err
	}

	return &QuestionListResult{
		Result:   items,
		PageIdx:  pageIdx,
		PageSize: pageSize,
		TotalCnt: total,
	}, nil
}

func (s *QuestionService) ListNewestPublished(pageIdx, pageSize int) ([]models.Question, error) {
	items := make([]models.Question, 0, pageSize)
	if err := s.db.Where("status = ?", "published").Order("id DESC").Limit(pageSize).Offset(pageSize * (pageIdx - 1)).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *QuestionService) Create(input QuestionCreateInput) (*models.Question, error) {
	questionID := strings.TrimSpace(input.QuestionID)
	if questionID == "" {
		return nil, ErrQuestionIDRequired
	}

	question := models.Question{
		QuestionId:  questionID,
		Title:       input.Title,
		Content:     input.Content,
		Difficulty:  input.Difficulty,
		TimeLimit:   input.TimeLimit,
		MemoryLimit: input.MemoryLimit,
		Tags:        input.Tags,
		Status:      input.Status,
	}

	if question.Status == "" {
		question.Status = "draft"
	}
	if question.TimeLimit == 0 {
		question.TimeLimit = 2000
	}
	if question.MemoryLimit == 0 {
		question.MemoryLimit = 256
	}

	if err := s.db.Create(&question).Error; err != nil {
		return nil, err
	}

	return &question, nil
}

func (s *QuestionService) GetByID(id int) (*models.Question, error) {
	var question models.Question
	if err := s.db.Where("id = ?", id).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionRecordNotFound
		}
		return nil, err
	}
	return &question, nil
}

func (s *QuestionService) GetByNumber(questionNumber int) (*models.Question, error) {
	return s.GetByID(questionNumber)
}

func (s *QuestionService) UpdateByID(id int, payload models.Question) (*models.Question, error) {
	var existing models.Question
	if err := s.db.Where("id = ?", id).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionRecordNotFound
		}
		return nil, err
	}

	payload.Id = 0

	if err := s.db.Model(&existing).Updates(payload).Error; err != nil {
		return nil, err
	}
	if err := s.db.Where("id = ?", existing.Id).First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *QuestionService) UpdateByNumber(questionNumber int, payload models.Question) (*models.Question, error) {
	return s.UpdateByID(questionNumber, payload)
}

func (s *QuestionService) DeleteByID(id int) error {
	var question models.Question
	if err := s.db.Where("id = ?", id).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrQuestionRecordNotFound
		}
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", question.Id).Delete(&models.TestCase{}).Error; err != nil {
			return err
		}
		return tx.Delete(&question).Error
	})
}

func (s *QuestionService) DeleteByNumber(questionNumber int) error {
	return s.DeleteByID(questionNumber)
}
