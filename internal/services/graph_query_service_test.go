package services

import (
	"context"
	"errors"
	"testing"

	"patreonoj/internal/graph"
)

type fakeGraphQueryStore struct {
	prerequisites          []graph.QuestionNode
	nextQuestions          []graph.QuestionNode
	learningPath           *graph.LearningPath
	recommendations        []graph.RecommendationResult
	questions              []graph.QuestionNode
	skills                 []graph.SkillNode
	questionRelations      []graph.QuestionRelation
	questionSkillRelations []graph.QuestionSkillRelation
	skillRelations         []graph.SkillRelation

	errQuestions              error
	errSkills                 error
	errQuestionRelations      error
	errQuestionSkillRelations error
	errSkillRelations         error
}

func (f *fakeGraphQueryStore) GetPrerequisites(_ context.Context, _ int) ([]graph.QuestionNode, error) {
	return f.prerequisites, nil
}

func (f *fakeGraphQueryStore) GetNextLevelQuestions(_ context.Context, _ int) ([]graph.QuestionNode, error) {
	return f.nextQuestions, nil
}

func (f *fakeGraphQueryStore) FindLearningPath(_ context.Context, _, _ int) (*graph.LearningPath, error) {
	return f.learningPath, nil
}

func (f *fakeGraphQueryStore) GetRecommendations(_ context.Context, _ int, _ int) ([]graph.RecommendationResult, error) {
	return f.recommendations, nil
}

func (f *fakeGraphQueryStore) ListQuestions(_ context.Context) ([]graph.QuestionNode, error) {
	return f.questions, f.errQuestions
}

func (f *fakeGraphQueryStore) ListSkills(_ context.Context) ([]graph.SkillNode, error) {
	return f.skills, f.errSkills
}

func (f *fakeGraphQueryStore) ListRelations(_ context.Context) ([]graph.QuestionRelation, error) {
	return f.questionRelations, f.errQuestionRelations
}

func (f *fakeGraphQueryStore) ListQuestionSkillRelations(_ context.Context) ([]graph.QuestionSkillRelation, error) {
	return f.questionSkillRelations, f.errQuestionSkillRelations
}

func (f *fakeGraphQueryStore) ListSkillRelations(_ context.Context) ([]graph.SkillRelation, error) {
	return f.skillRelations, f.errSkillRelations
}

func TestGraphQueryService_ListGraphData_BuildsEdges(t *testing.T) {
	store := &fakeGraphQueryStore{
		questions: []graph.QuestionNode{
			{QuestionNumber: 1001, QuestionId: "q-a", Title: "A"},
			{QuestionNumber: 1002, QuestionId: "", Title: "B"},
		},
		skills: []graph.SkillNode{
			{Key: "dp", Name: "DP"},
		},
		questionRelations: []graph.QuestionRelation{
			{FromQuestionNumber: 1001, ToQuestionNumber: 1002, RelationType: graph.SIMILAR, Weight: 0.7},
		},
		questionSkillRelations: []graph.QuestionSkillRelation{
			{QuestionNumber: 1002, SkillKey: "dp", Weight: 1.0},
		},
		skillRelations: []graph.SkillRelation{
			{FromKey: "dp", ToKey: "graph", RelationType: graph.SKILL_SUBSUMES, Weight: 0.6},
		},
	}
	svc := NewGraphQueryService(store)

	result, err := svc.ListGraphData(context.Background())
	if err != nil {
		t.Fatalf("ListGraphData failed: %v", err)
	}

	if result.Count != 2 || result.SkillCount != 1 || result.EdgeCount != 3 {
		t.Fatalf("unexpected counters: count=%d skill=%d edge=%d", result.Count, result.SkillCount, result.EdgeCount)
	}
	if len(result.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(result.Edges))
	}
	if result.Edges[0].From != "Q:q-a" || result.Edges[0].To != "Q:1002" {
		t.Fatalf("unexpected question relation edge: %+v", result.Edges[0])
	}
	if result.Edges[1].From != "Q:1002" || result.Edges[1].To != "S:dp" {
		t.Fatalf("unexpected question-skill edge: %+v", result.Edges[1])
	}
	if result.Edges[2].From != "S:dp" || result.Edges[2].To != "S:graph" {
		t.Fatalf("unexpected skill edge: %+v", result.Edges[2])
	}
}

func TestGraphQueryService_ListGraphData_StageError(t *testing.T) {
	svc := NewGraphQueryService(&fakeGraphQueryStore{
		errQuestionSkillRelations: errors.New("boom"),
	})

	_, err := svc.ListGraphData(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}

	var qerr *GraphQueryError
	if !errors.As(err, &qerr) {
		t.Fatalf("expected GraphQueryError, got %T", err)
	}
	if qerr.Stage != GraphListStageQuestionSkillRelations {
		t.Fatalf("expected stage %s, got %s", GraphListStageQuestionSkillRelations, qerr.Stage)
	}
}
