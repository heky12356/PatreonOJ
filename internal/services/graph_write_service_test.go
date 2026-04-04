package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"dachuang/internal/graph"
	"dachuang/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeGraphWriteStore struct {
	createOrUpdateErr error
	syncSkillsErr     error
	buildSimilarErr   error
	createRelationErr error
	deleteRelationErr error

	lastQuestionNode graph.QuestionNode
	lastRelation     graph.QuestionRelation
	deleteArgs       struct {
		from int
		to   int
		typ  graph.RelationshipType
	}
}

func (f *fakeGraphWriteStore) CreateOrUpdateQuestion(_ context.Context, question graph.QuestionNode) error {
	f.lastQuestionNode = question
	return f.createOrUpdateErr
}

func (f *fakeGraphWriteStore) SyncQuestionSkills(_ context.Context, _ int, _ string, _ time.Time) error {
	return f.syncSkillsErr
}

func (f *fakeGraphWriteStore) BuildTagSimilarEdgesForQuestion(_ context.Context, _ int, _ time.Time) error {
	return f.buildSimilarErr
}

func (f *fakeGraphWriteStore) CreateRelation(_ context.Context, relation graph.QuestionRelation) error {
	f.lastRelation = relation
	return f.createRelationErr
}

func (f *fakeGraphWriteStore) DeleteRelation(_ context.Context, fromQuestion, toQuestion int, relationType graph.RelationshipType) error {
	f.deleteArgs = struct {
		from int
		to   int
		typ  graph.RelationshipType
	}{from: fromQuestion, to: toQuestion, typ: relationType}
	return f.deleteRelationErr
}

func newWriteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Question{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedQuestion(t *testing.T, db *gorm.DB, number int) {
	t.Helper()
	err := db.Create(&models.Question{
		QuestionNumber: number,
		QuestionId:     "qid",
		Title:          "title",
		Difficulty:     "Easy",
		Tags:           "dp,array",
		Status:         "published",
	}).Error
	if err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
}

func TestGraphWriteService_SyncQuestion_NotFound(t *testing.T) {
	db := newWriteTestDB(t)
	svc := NewGraphWriteService(db, &fakeGraphWriteStore{})

	err := svc.SyncQuestion(context.Background(), 9999)
	if !errors.Is(err, ErrSyncQuestionNotFound) {
		t.Fatalf("expected ErrSyncQuestionNotFound, got %v", err)
	}
}

func TestGraphWriteService_SyncQuestion_StageError(t *testing.T) {
	db := newWriteTestDB(t)
	seedQuestion(t, db, 1001)
	svc := NewGraphWriteService(db, &fakeGraphWriteStore{
		syncSkillsErr: errors.New("x"),
	})

	err := svc.SyncQuestion(context.Background(), 1001)
	var syncErr *GraphSyncError
	if !errors.As(err, &syncErr) {
		t.Fatalf("expected GraphSyncError, got %v", err)
	}
	if syncErr.Stage != GraphSyncStageSyncSkills {
		t.Fatalf("expected stage %s, got %s", GraphSyncStageSyncSkills, syncErr.Stage)
	}
}

func TestGraphWriteService_SyncQuestion_Success(t *testing.T) {
	db := newWriteTestDB(t)
	seedQuestion(t, db, 1001)
	store := &fakeGraphWriteStore{}
	svc := NewGraphWriteService(db, store)

	err := svc.SyncQuestion(context.Background(), 1001)
	if err != nil {
		t.Fatalf("SyncQuestion failed: %v", err)
	}
	if store.lastQuestionNode.QuestionNumber != 1001 {
		t.Fatalf("unexpected question number: %d", store.lastQuestionNode.QuestionNumber)
	}
	if store.lastQuestionNode.CreatedAt.IsZero() || store.lastQuestionNode.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set")
	}
}

func TestGraphWriteService_CreateRelation_InvalidType(t *testing.T) {
	svc := NewGraphWriteService(newWriteTestDB(t), &fakeGraphWriteStore{})
	err := svc.CreateRelation(context.Background(), 1, 2, graph.RelationshipType("UNKNOWN"), 1, "")
	if !errors.Is(err, ErrInvalidRelationType) {
		t.Fatalf("expected ErrInvalidRelationType, got %v", err)
	}
}

func TestGraphWriteService_CreateRelation_Success(t *testing.T) {
	store := &fakeGraphWriteStore{}
	svc := NewGraphWriteService(newWriteTestDB(t), store)
	err := svc.CreateRelation(context.Background(), 1, 2, graph.SIMILAR, 0.8, "r")
	if err != nil {
		t.Fatalf("CreateRelation failed: %v", err)
	}
	if store.lastRelation.FromQuestionNumber != 1 || store.lastRelation.ToQuestionNumber != 2 {
		t.Fatalf("unexpected relation: %+v", store.lastRelation)
	}
	if store.lastRelation.CreatedAt.IsZero() {
		t.Fatalf("expected created_at set")
	}
}

func TestGraphWriteService_DeleteRelation_PassThrough(t *testing.T) {
	store := &fakeGraphWriteStore{}
	svc := NewGraphWriteService(newWriteTestDB(t), store)
	err := svc.DeleteRelation(context.Background(), 10, 20, graph.PREREQUISITE)
	if err != nil {
		t.Fatalf("DeleteRelation failed: %v", err)
	}
	if store.deleteArgs.from != 10 || store.deleteArgs.to != 20 || store.deleteArgs.typ != graph.PREREQUISITE {
		t.Fatalf("unexpected delete args: %+v", store.deleteArgs)
	}
}
