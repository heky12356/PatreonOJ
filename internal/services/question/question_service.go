package question

import (
	"errors"
	"strconv"
	"strings"
	"sync"

	"patreonoj/internal/graphcache"
	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var mutationMu sync.Mutex

var (
	ErrQuestionRecordNotFound = errors.New("question not found")
	ErrQuestionIDRequired     = errors.New("question_id is required")
	ErrQuestionInvalidInput   = errors.New("invalid question input")
	ErrQuestionBusy           = errors.New("question has unfinished submissions")
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

type QuestionUpdateInput struct {
	QuestionID  *string `json:"question_id"`
	Title       *string `json:"title"`
	Content     *string `json:"content"`
	Difficulty  *string `json:"difficulty"`
	TimeLimit   *int    `json:"time_limit"`
	MemoryLimit *int    `json:"memory_limit"`
	Tags        *string `json:"tags"`
	Status      *string `json:"status"`
}

func validateQuestion(q models.Question) error {
	if strings.TrimSpace(q.QuestionId) == "" || strings.TrimSpace(q.Title) == "" {
		return ErrQuestionInvalidInput
	}
	if q.Status != "draft" && q.Status != "published" {
		return ErrQuestionInvalidInput
	}
	if q.TimeLimit <= 0 || q.TimeLimit > 600000 || q.MemoryLimit <= 0 || q.MemoryLimit > 8192 {
		return ErrQuestionInvalidInput
	}
	return nil
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
	mutationMu.Lock()
	defer mutationMu.Unlock()
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

	if err := validateQuestion(question); err != nil {
		return nil, err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&question).Error; err != nil {
			return err
		}
		return refreshQuestionEdges(tx, question.Id)
	}); err != nil {
		return nil, err
	}

	graphcache.Invalidate()
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
	// Preserve callers of the old partial-struct API; HTTP requests use pointers
	// below so explicitly empty content or tags can be distinguished from omission.
	input := QuestionUpdateInput{}
	if payload.QuestionId != "" {
		input.QuestionID = &payload.QuestionId
	}
	if payload.Title != "" {
		input.Title = &payload.Title
	}
	if payload.Content != "" {
		input.Content = &payload.Content
	}
	if payload.Difficulty != "" {
		input.Difficulty = &payload.Difficulty
	}
	if payload.TimeLimit != 0 {
		input.TimeLimit = &payload.TimeLimit
	}
	if payload.MemoryLimit != 0 {
		input.MemoryLimit = &payload.MemoryLimit
	}
	if payload.Tags != "" {
		input.Tags = &payload.Tags
	}
	if payload.Status != "" {
		input.Status = &payload.Status
	}
	return s.UpdateFieldsByID(id, input)
}

func (s *QuestionService) UpdateFieldsByID(id int, input QuestionUpdateInput) (*models.Question, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	var existing models.Question
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrQuestionRecordNotFound
			}
			return err
		}
		oldID := existing.QuestionId
		oldTags, oldDifficulty, oldStatus := existing.Tags, existing.Difficulty, existing.Status
		if input.QuestionID != nil {
			existing.QuestionId = strings.TrimSpace(*input.QuestionID)
		}
		if input.Title != nil {
			existing.Title = *input.Title
		}
		if input.Content != nil {
			existing.Content = *input.Content
		}
		if input.Difficulty != nil {
			existing.Difficulty = *input.Difficulty
		}
		if input.TimeLimit != nil {
			existing.TimeLimit = *input.TimeLimit
		}
		if input.MemoryLimit != nil {
			existing.MemoryLimit = *input.MemoryLimit
		}
		if input.Tags != nil {
			existing.Tags = *input.Tags
		}
		if input.Status != nil {
			existing.Status = strings.TrimSpace(*input.Status)
		}
		if err := validateQuestion(existing); err != nil {
			return err
		}
		if err := tx.Model(&existing).Select("question_id", "title", "content", "difficulty", "time_limit", "memory_limit", "tags", "status").Updates(existing).Error; err != nil {
			return err
		}
		if oldID != existing.QuestionId {
			if err := deleteQuestionEdges(tx, oldID); err != nil {
				return err
			}
		}
		if oldID != existing.QuestionId || oldTags != existing.Tags || oldDifficulty != existing.Difficulty || oldStatus != existing.Status {
			return refreshQuestionEdges(tx, existing.Id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	graphcache.Invalidate()
	return &existing, nil
}

func (s *QuestionService) UpdateByNumber(questionNumber int, payload models.Question) (*models.Question, error) {
	return s.UpdateByID(questionNumber, payload)
}

func (s *QuestionService) DeleteByID(id int) error {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	var question models.Question
	if err := s.db.Where("id = ?", id).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrQuestionRecordNotFound
		}
		return err
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&models.Submission{}) {
			var pending int64
			if err := tx.Model(&models.Submission{}).Where("question_id = ? AND status IN ?", question.Id, []string{"pending", "processing"}).Count(&pending).Error; err != nil {
				return err
			}
			if pending > 0 {
				return ErrQuestionBusy
			}
		}
		if err := tx.Where("question_id = ?", question.Id).Delete(&models.TestCase{}).Error; err != nil {
			return err
		}
		if err := tx.Where("question_id = ?", question.Id).Delete(&models.QuestionTag{}).Error; err != nil {
			return err
		}
		if err := deleteQuestionEdges(tx, question.QuestionId); err != nil {
			return err
		}
		return tx.Delete(&question).Error
	})
	if err == nil {
		graphcache.Invalidate()
	}
	return err
}

func (s *QuestionService) DeleteByNumber(questionNumber int) error {
	return s.DeleteByID(questionNumber)
}
