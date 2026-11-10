package testcase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"patreonoj/internal/models"

	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

var (
	ErrTestCaseQuestionNotFound = errors.New("question not found")
	ErrTestCaseNotFound         = errors.New("test case not found")
	ErrTestCaseOSSUnavailable   = errors.New("oss unavailable")
	ErrTestCaseInputKeyInvalid  = errors.New("input key invalid")
	ErrTestCaseOutputKeyInvalid = errors.New("output key invalid")
)

type testCaseObjectStore interface {
	StatObject(ctx context.Context, bucket, key string) (minio.ObjectInfo, error)
}

type TestCaseService struct {
	db        *gorm.DB
	ossClient testCaseObjectStore
}

type TestCaseOSSCommitInput struct {
	QuestionPK int
	QuestionID string
	InputKey   string
	OutputKey  string
	IsHidden   bool
}

func NewTestCaseService(db *gorm.DB, ossClient testCaseObjectStore) *TestCaseService {
	return &TestCaseService{
		db:        db,
		ossClient: ossClient,
	}
}

func (s *TestCaseService) List(questionID *int) ([]models.TestCase, error) {
	query := s.db
	if questionID != nil {
		question, err := s.getQuestionByID(*questionID)
		if err != nil {
			return nil, err
		}
		query = query.Where("question_id = ?", question.Id)
	}

	return s.listByQuery(query)
}

func (s *TestCaseService) GetByQuestionID(questionID int) ([]models.TestCase, models.Question, error) {
	question, err := s.getQuestionByID(questionID)
	if err != nil {
		return nil, models.Question{}, err
	}

	var testCases []models.TestCase
	if err := s.db.Where("question_id = ?", question.Id).Order("id ASC").Find(&testCases).Error; err != nil {
		return nil, models.Question{}, err
	}

	return testCases, question, nil
}

func (s *TestCaseService) CommitOSS(ctx context.Context, bucket string, input TestCaseOSSCommitInput) (models.TestCase, error) {
	if s.ossClient == nil {
		return models.TestCase{}, ErrTestCaseOSSUnavailable
	}

	question, err := s.resolveQuestion(input.QuestionPK, input.QuestionID)
	if err != nil {
		return models.TestCase{}, err
	}

	inInfo, outInfo, err := s.validateObjectKeys(ctx, bucket, question.Id, input.InputKey, input.OutputKey)
	if err != nil {
		return models.TestCase{}, err
	}

	testCase := models.TestCase{
		QuestionID: question.Id,
		InputKey:   input.InputKey,
		OutputKey:  input.OutputKey,
		InputSize:  inInfo.Size,
		OutputSize: outInfo.Size,
		IsHidden:   input.IsHidden,
	}

	if err := s.db.Create(&testCase).Error; err != nil {
		return models.TestCase{}, err
	}
	return testCase, nil
}

func (s *TestCaseService) UpdateOSS(ctx context.Context, bucket string, id uint, input TestCaseOSSCommitInput) (models.TestCase, error) {
	if s.ossClient == nil {
		return models.TestCase{}, ErrTestCaseOSSUnavailable
	}

	testCase, err := s.GetByID(id)
	if err != nil {
		return models.TestCase{}, err
	}

	question, err := s.resolveQuestion(input.QuestionPK, input.QuestionID)
	if err != nil {
		return models.TestCase{}, err
	}

	inInfo, outInfo, err := s.validateObjectKeys(ctx, bucket, question.Id, input.InputKey, input.OutputKey)
	if err != nil {
		return models.TestCase{}, err
	}

	testCase.QuestionID = question.Id
	testCase.InputKey = input.InputKey
	testCase.OutputKey = input.OutputKey
	testCase.InputSize = inInfo.Size
	testCase.OutputSize = outInfo.Size
	testCase.IsHidden = input.IsHidden

	if err := s.db.Save(&testCase).Error; err != nil {
		return models.TestCase{}, err
	}
	return testCase, nil
}

func (s *TestCaseService) GetByID(id uint) (models.TestCase, error) {
	var testCase models.TestCase
	if err := s.db.Where("id = ?", id).First(&testCase).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.TestCase{}, ErrTestCaseNotFound
		}
		return models.TestCase{}, err
	}
	return testCase, nil
}

func (s *TestCaseService) Delete(id uint) error {
	testCase, err := s.GetByID(id)
	if err != nil {
		return err
	}
	return s.db.Delete(&testCase).Error
}

func (s *TestCaseService) getQuestionByID(questionID int) (models.Question, error) {
	var question models.Question
	if err := s.db.Where("id = ?", questionID).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Question{}, ErrTestCaseQuestionNotFound
		}
		return models.Question{}, err
	}
	return question, nil
}

func (s *TestCaseService) getQuestionByQuestionID(questionID string) (models.Question, error) {
	var question models.Question
	if err := s.db.Where("question_id = ?", questionID).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Question{}, ErrTestCaseQuestionNotFound
		}
		return models.Question{}, err
	}
	return question, nil
}

func (s *TestCaseService) resolveQuestion(questionPK int, questionID string) (models.Question, error) {
	if questionPK > 0 {
		return s.getQuestionByID(questionPK)
	}
	if id := strings.TrimSpace(questionID); id != "" {
		return s.getQuestionByQuestionID(id)
	}
	return models.Question{}, ErrTestCaseQuestionNotFound
}

func (s *TestCaseService) validateObjectKeys(ctx context.Context, bucket string, questionPK int, inputKey, outputKey string) (minio.ObjectInfo, minio.ObjectInfo, error) {
	expectedPrefix := fmt.Sprintf("problems/%d/", questionPK)
	if err := ensureQuestionPrefix(expectedPrefix, inputKey); err != nil {
		return minio.ObjectInfo{}, minio.ObjectInfo{}, fmt.Errorf("%w: %v", ErrTestCaseInputKeyInvalid, err)
	}
	if err := ensureQuestionPrefix(expectedPrefix, outputKey); err != nil {
		return minio.ObjectInfo{}, minio.ObjectInfo{}, fmt.Errorf("%w: %v", ErrTestCaseOutputKeyInvalid, err)
	}

	inInfo, err := s.ossClient.StatObject(ctx, bucket, inputKey)
	if err != nil {
		return minio.ObjectInfo{}, minio.ObjectInfo{}, fmt.Errorf("%w: %v", ErrTestCaseInputKeyInvalid, err)
	}
	outInfo, err := s.ossClient.StatObject(ctx, bucket, outputKey)
	if err != nil {
		return minio.ObjectInfo{}, minio.ObjectInfo{}, fmt.Errorf("%w: %v", ErrTestCaseOutputKeyInvalid, err)
	}
	return inInfo, outInfo, nil
}

func ensureQuestionPrefix(expectedPrefix, key string) error {
	trimmed := strings.TrimLeft(strings.TrimSpace(key), "/")
	if trimmed == "" {
		return errors.New("key is empty")
	}
	if !strings.HasPrefix(trimmed, expectedPrefix) {
		return fmt.Errorf("key must start with %q", expectedPrefix)
	}
	return nil
}

func (s *TestCaseService) listByQuery(query *gorm.DB) ([]models.TestCase, error) {
	var testCases []models.TestCase
	if err := query.Order("id ASC").Find(&testCases).Error; err != nil {
		return nil, err
	}
	return testCases, nil
}
