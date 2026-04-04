package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"patreonoj/internal/graph"
	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var (
	ErrSyncQuestionNotFound = errors.New("question not found")
	ErrInvalidRelationType  = errors.New("invalid relation type")
)

type graphWriteStore interface {
	CreateOrUpdateQuestion(ctx context.Context, question graph.QuestionNode) error
	SyncQuestionSkills(ctx context.Context, questionNumber int, tags string, now time.Time) error
	BuildTagSimilarEdgesForQuestion(ctx context.Context, questionNumber int, now time.Time) error
	CreateRelation(ctx context.Context, relation graph.QuestionRelation) error
	DeleteRelation(ctx context.Context, fromQuestion, toQuestion int, relationType graph.RelationshipType) error
}

type GraphSyncStage string

const (
	GraphSyncStageQueryQuestion    GraphSyncStage = "query_question"
	GraphSyncStageUpsertQuestion   GraphSyncStage = "upsert_question"
	GraphSyncStageSyncSkills       GraphSyncStage = "sync_skills"
	GraphSyncStageBuildSimilarEdge GraphSyncStage = "build_similar_edge"
)

type GraphSyncError struct {
	Stage GraphSyncStage
	Err   error
}

func (e *GraphSyncError) Error() string {
	return fmt.Sprintf("graph sync failed at stage %s: %v", e.Stage, e.Err)
}

func (e *GraphSyncError) Unwrap() error {
	return e.Err
}

type GraphWriteService struct {
	db    *gorm.DB
	store graphWriteStore
}

func NewGraphWriteService(db *gorm.DB, store graphWriteStore) *GraphWriteService {
	return &GraphWriteService{db: db, store: store}
}

func (s *GraphWriteService) SyncQuestion(ctx context.Context, questionNumber int) error {
	var question models.Question
	if err := s.db.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSyncQuestionNotFound
		}
		return &GraphSyncError{Stage: GraphSyncStageQueryQuestion, Err: err}
	}

	now := time.Now().UTC()
	questionNode := graph.QuestionNode{
		QuestionNumber: question.QuestionNumber,
		QuestionId:     question.QuestionId,
		Title:          question.Title,
		Difficulty:     question.Difficulty,
		Tags:           question.Tags,
		Status:         question.Status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.store.CreateOrUpdateQuestion(ctx, questionNode); err != nil {
		return &GraphSyncError{Stage: GraphSyncStageUpsertQuestion, Err: err}
	}
	if err := s.store.SyncQuestionSkills(ctx, question.QuestionNumber, question.Tags, now); err != nil {
		return &GraphSyncError{Stage: GraphSyncStageSyncSkills, Err: err}
	}
	if err := s.store.BuildTagSimilarEdgesForQuestion(ctx, question.QuestionNumber, now); err != nil {
		return &GraphSyncError{Stage: GraphSyncStageBuildSimilarEdge, Err: err}
	}
	return nil
}

func (s *GraphWriteService) CreateRelation(ctx context.Context, fromQuestion, toQuestion int, relationType graph.RelationshipType, weight float64, description string) error {
	if !validRelationType(relationType) {
		return ErrInvalidRelationType
	}

	relation := graph.QuestionRelation{
		FromQuestionNumber: fromQuestion,
		ToQuestionNumber:   toQuestion,
		RelationType:       relationType,
		Weight:             weight,
		Description:        description,
		CreatedAt:          time.Now().UTC(),
	}
	return s.store.CreateRelation(ctx, relation)
}

func (s *GraphWriteService) DeleteRelation(ctx context.Context, fromQuestion, toQuestion int, relationType graph.RelationshipType) error {
	return s.store.DeleteRelation(ctx, fromQuestion, toQuestion, relationType)
}

func validRelationType(relationType graph.RelationshipType) bool {
	switch relationType {
	case graph.PREREQUISITE, graph.NEXT_LEVEL, graph.SIMILAR, graph.CATEGORY:
		return true
	default:
		return false
	}
}
