package services

import (
	"errors"
	"strings"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var (
	ErrQuestionRecordNotFound = errors.New("question not found")
	ErrQuestionNumberExists   = errors.New("question number already exists")
)

type QuestionListResult struct {
	Result   []models.Question `json:"result"`
	PageIdx  int               `json:"pageIdx"`
	PageSize int               `json:"pageSize"`
	TotalCnt int64             `json:"totalCnt"`
}

type QuestionCreateInput struct {
	QuestionNumber int
	QuestionID     string
	Title          string
	Content        string
	Difficulty     string
	DataRange      string
	TimeLimit      int
	MemoryLimit    int
	Tags           string
	CategoryID     int
	Status         string
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

	base := s.db.Model(&models.Question{}).
		Where("difficulty LIKE ? AND (title LIKE ? OR question_number LIKE ?) AND status LIKE ?", diff, query, query, status)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	items := make([]models.Question, 0, pageSize)
	if err := base.Order("question_number ASC").Limit(pageSize).Offset(pageSize * (pageIdx - 1)).Find(&items).Error; err != nil {
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
	question := models.Question{
		QuestionNumber: input.QuestionNumber,
		QuestionId:     input.QuestionID,
		Title:          input.Title,
		Content:        input.Content,
		Difficulty:     input.Difficulty,
		DataRange:      input.DataRange,
		TimeLimit:      input.TimeLimit,
		MemoryLimit:    input.MemoryLimit,
		Tags:           input.Tags,
		Category_id:    input.CategoryID,
		Status:         input.Status,
	}

	if question.QuestionNumber == 0 {
		var maxNumber int
		if err := s.db.Model(&models.Question{}).Select("COALESCE(MAX(question_number), 1000)").Scan(&maxNumber).Error; err != nil {
			return nil, err
		}
		question.QuestionNumber = maxNumber + 1
	} else {
		var existing models.Question
		if err := s.db.Where("question_number = ?", question.QuestionNumber).First(&existing).Error; err == nil {
			return nil, ErrQuestionNumberExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
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

func (s *QuestionService) GetByNumber(questionNumber int) (*models.Question, error) {
	var question models.Question
	if err := s.db.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionRecordNotFound
		}
		return nil, err
	}
	return &question, nil
}

func (s *QuestionService) GetByQuestionID(questionID string) (*models.Question, error) {
	var question models.Question
	if err := s.db.Where("question_id = ?", questionID).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionRecordNotFound
		}
		return nil, err
	}
	return &question, nil
}

func (s *QuestionService) UpdateByNumber(questionNumber int, payload models.Question) (*models.Question, error) {
	var existing models.Question
	if err := s.db.Where("question_number = ?", questionNumber).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionRecordNotFound
		}
		return nil, err
	}

	if err := s.db.Model(&existing).Updates(payload).Error; err != nil {
		return nil, err
	}
	if err := s.db.Where("question_number = ?", questionNumber).First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *QuestionService) DeleteByNumber(questionNumber int) error {
	var question models.Question
	if err := s.db.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrQuestionRecordNotFound
		}
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", question.Id).Delete(&models.TestCase{}).Error; err != nil {
			return err
		}
		if err := tx.Where("question_id = ?", question.Id).Delete(&models.QuestionCategory{}).Error; err != nil {
			return err
		}
		return tx.Delete(&question).Error
	})
}
