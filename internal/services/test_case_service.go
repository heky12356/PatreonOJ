package services

import (
	"context"
	"errors"
	"fmt"

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

type TestCaseCreateInput struct {
	QuestionNumber int
	Input          string
	ExpectedOutput string
	IsHidden       bool
}

type TestCaseBatchItem struct {
	Input          string
	ExpectedOutput string
	IsHidden       bool
}

type TestCaseOSSCommitInput struct {
	QuestionNumber int
	InputKey       string
	OutputKey      string
	IsHidden       bool
}

func NewTestCaseService(db *gorm.DB, ossClient testCaseObjectStore) *TestCaseService {
	return &TestCaseService{
		db:        db,
		ossClient: ossClient,
	}
}

func (s *TestCaseService) List(questionNumber *int) ([]models.TestCase, error) {
	query := s.db
	if questionNumber != nil {
		question, err := s.getQuestionByNumber(*questionNumber)
		if err != nil {
			return nil, err
		}
		query = query.Where("question_id = ?", question.Id)
	}

	var testCases []models.TestCase
	if err := query.Order("id ASC").Find(&testCases).Error; err != nil {
		return nil, err
	}
	return testCases, nil
}

func (s *TestCaseService) GetByQuestionNumber(questionNumber int) ([]models.TestCase, models.Question, error) {
	question, err := s.getQuestionByNumber(questionNumber)
	if err != nil {
		return nil, models.Question{}, err
	}

	var testCases []models.TestCase
	if err := s.db.Where("question_id = ?", question.Id).Order("id ASC").Find(&testCases).Error; err != nil {
		return nil, models.Question{}, err
	}

	return testCases, question, nil
}

func (s *TestCaseService) Create(input TestCaseCreateInput) (models.TestCase, error) {
	question, err := s.getQuestionByNumber(input.QuestionNumber)
	if err != nil {
		return models.TestCase{}, err
	}

	testCase := models.TestCase{
		QuestionID:     question.Id,
		Input:          input.Input,
		ExpectedOutput: input.ExpectedOutput,
		IsHidden:       input.IsHidden,
	}
	if err := s.db.Create(&testCase).Error; err != nil {
		return models.TestCase{}, err
	}
	return testCase, nil
}

func (s *TestCaseService) BatchCreate(questionNumber int, items []TestCaseBatchItem) ([]models.TestCase, error) {
	question, err := s.getQuestionByNumber(questionNumber)
	if err != nil {
		return nil, err
	}

	testCases := make([]models.TestCase, 0, len(items))
	for _, item := range items {
		testCases = append(testCases, models.TestCase{
			QuestionID:     question.Id,
			Input:          item.Input,
			ExpectedOutput: item.ExpectedOutput,
			IsHidden:       item.IsHidden,
		})
	}

	if err := s.db.Create(&testCases).Error; err != nil {
		return nil, err
	}
	return testCases, nil
}

func (s *TestCaseService) CommitOSS(ctx context.Context, bucket string, input TestCaseOSSCommitInput) (models.TestCase, error) {
	if s.ossClient == nil {
		return models.TestCase{}, ErrTestCaseOSSUnavailable
	}

	question, err := s.getQuestionByNumber(input.QuestionNumber)
	if err != nil {
		return models.TestCase{}, err
	}

	inInfo, err := s.ossClient.StatObject(ctx, bucket, input.InputKey)
	if err != nil {
		return models.TestCase{}, fmt.Errorf("%w: %v", ErrTestCaseInputKeyInvalid, err)
	}
	outInfo, err := s.ossClient.StatObject(ctx, bucket, input.OutputKey)
	if err != nil {
		return models.TestCase{}, fmt.Errorf("%w: %v", ErrTestCaseOutputKeyInvalid, err)
	}

	testCase := models.TestCase{
		QuestionID:     question.Id,
		InputKey:       input.InputKey,
		OutputKey:      input.OutputKey,
		InputSize:      inInfo.Size,
		OutputSize:     outInfo.Size,
		IsHidden:       input.IsHidden,
		Input:          "",
		ExpectedOutput: "",
	}

	if err := s.db.Create(&testCase).Error; err != nil {
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

func (s *TestCaseService) Update(id uint, input TestCaseCreateInput) (models.TestCase, error) {
	testCase, err := s.GetByID(id)
	if err != nil {
		return models.TestCase{}, err
	}

	question, err := s.getQuestionByNumber(input.QuestionNumber)
	if err != nil {
		return models.TestCase{}, err
	}

	testCase.QuestionID = question.Id
	testCase.Input = input.Input
	testCase.ExpectedOutput = input.ExpectedOutput
	testCase.IsHidden = input.IsHidden

	if err := s.db.Save(&testCase).Error; err != nil {
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

func (s *TestCaseService) getQuestionByNumber(questionNumber int) (models.Question, error) {
	var question models.Question
	if err := s.db.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Question{}, ErrTestCaseQuestionNotFound
		}
		return models.Question{}, err
	}
	return question, nil
}
