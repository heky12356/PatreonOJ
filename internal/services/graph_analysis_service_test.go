package services

import (
	"context"
	"errors"
	"testing"

	"patreonoj/internal/graph"
	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeGraphStore struct {
	skills         []graph.SkillNode
	createRelErr   error
	createSkillErr error
	createRelCalls []graph.QuestionRelation
	skillRelCalls  []struct{ from, to, reason string }
}

func (f *fakeGraphStore) CreateRelation(_ context.Context, relation graph.QuestionRelation) error {
	if f.createRelErr != nil {
		return f.createRelErr
	}
	f.createRelCalls = append(f.createRelCalls, relation)
	return nil
}

func (f *fakeGraphStore) ListSkills(_ context.Context) ([]graph.SkillNode, error) {
	return f.skills, nil
}

func (f *fakeGraphStore) CreateSkillRelation(_ context.Context, fromKey, toKey string, _ graph.SkillRelationType, reason string) error {
	if f.createSkillErr != nil {
		return f.createSkillErr
	}
	f.skillRelCalls = append(f.skillRelCalls, struct{ from, to, reason string }{from: fromKey, to: toKey, reason: reason})
	return nil
}

type fakeAI struct {
	relations      []QuestionRelation
	skillRelations []SkillRelation
	relationErr    error
	skillErr       error
}

func (f *fakeAI) AnalyzeQuestionRelations(_ context.Context, _ string, _ string) ([]QuestionRelation, error) {
	if f.relationErr != nil {
		return nil, f.relationErr
	}
	return f.relations, nil
}

func (f *fakeAI) AnalyzeSkillTree(_ context.Context, _ []string) ([]SkillRelation, error) {
	if f.skillErr != nil {
		return nil, f.skillErr
	}
	return f.skillRelations, nil
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Question{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func createQuestion(t *testing.T, db *gorm.DB, number int, title, difficulty, tags string) {
	t.Helper()
	err := db.Create(&models.Question{
		QuestionNumber: number,
		QuestionId:     "",
		Title:          title,
		Difficulty:     difficulty,
		Tags:           tags,
		Status:         "published",
	}).Error
	if err != nil {
		t.Fatalf("create question failed: %v", err)
	}
}

func TestGraphAnalysisService_AnalyzeQuestionRelations_NotFound(t *testing.T) {
	db := newTestDB(t)
	svc := NewGraphAnalysisService(db, &fakeGraphStore{}, &fakeAI{})

	_, _, err := svc.AnalyzeQuestionRelations(context.Background(), 9999)
	if !errors.Is(err, ErrQuestionNotFound) {
		t.Fatalf("expected ErrQuestionNotFound, got: %v", err)
	}
}

func TestGraphAnalysisService_AnalyzeQuestionRelations_SaveCount(t *testing.T) {
	db := newTestDB(t)
	createQuestion(t, db, 1001, "A", "Easy", "dp, array")
	createQuestion(t, db, 1002, "B", "Easy", "dp")
	createQuestion(t, db, 1003, "C", "Medium", "graph")

	store := &fakeGraphStore{}
	ai := &fakeAI{
		relations: []QuestionRelation{
			{SourceID: 1002, TargetID: 0, RelationType: "PREREQUISITE", Reason: "base"},
			{SourceID: 1003, TargetID: 1001, RelationType: "SIMILAR", Reason: "similar"},
			{SourceID: 1003, TargetID: 1001, RelationType: "UNKNOWN", Reason: "skip"},
		},
	}
	svc := NewGraphAnalysisService(db, store, ai)

	relations, saved, err := svc.AnalyzeQuestionRelations(context.Background(), 1001)
	if err != nil {
		t.Fatalf("AnalyzeQuestionRelations failed: %v", err)
	}
	if len(relations) != 3 {
		t.Fatalf("unexpected relations length: %d", len(relations))
	}
	if saved != 2 {
		t.Fatalf("expected saved=2, got %d", saved)
	}
	if len(store.createRelCalls) != 2 {
		t.Fatalf("expected 2 create relation calls, got %d", len(store.createRelCalls))
	}
	if store.createRelCalls[0].ToQuestionNumber != 1001 {
		t.Fatalf("expected default target question to be current question, got %d", store.createRelCalls[0].ToQuestionNumber)
	}
}

func TestGraphAnalysisService_AnalyzeSkillTree_NoSkills(t *testing.T) {
	db := newTestDB(t)
	svc := NewGraphAnalysisService(db, &fakeGraphStore{}, &fakeAI{})

	relations, saved, failed, hasSkills, err := svc.AnalyzeSkillTree(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeSkillTree failed: %v", err)
	}
	if hasSkills {
		t.Fatalf("expected hasSkills=false")
	}
	if len(relations) != 0 || saved != 0 || failed != 0 {
		t.Fatalf("unexpected result: relations=%d saved=%d failed=%d", len(relations), saved, failed)
	}
}

func TestGraphAnalysisService_AnalyzeSkillTree_SaveAndFailCount(t *testing.T) {
	db := newTestDB(t)
	store := &fakeGraphStore{
		skills: []graph.SkillNode{
			{Key: "dp", Name: "DP"},
			{Key: "array", Name: "Array"},
		},
	}
	ai := &fakeAI{
		skillRelations: []SkillRelation{
			{ParentSkill: "DP", ChildSkill: "Array", Reason: "r1"},
			{ParentSkill: "Unknown", ChildSkill: "Array", Reason: "r2"},
		},
	}
	svc := NewGraphAnalysisService(db, store, ai)

	relations, saved, failed, hasSkills, err := svc.AnalyzeSkillTree(context.Background())
	if err != nil {
		t.Fatalf("AnalyzeSkillTree failed: %v", err)
	}
	if !hasSkills {
		t.Fatalf("expected hasSkills=true")
	}
	if len(relations) != 2 {
		t.Fatalf("unexpected relations length: %d", len(relations))
	}
	if saved != 1 || failed != 1 {
		t.Fatalf("expected saved=1 failed=1, got saved=%d failed=%d", saved, failed)
	}
	if len(store.skillRelCalls) != 1 {
		t.Fatalf("expected one CreateSkillRelation call, got %d", len(store.skillRelCalls))
	}
}
