package services

import (
	"context"
	"fmt"
	"strconv"

	"dachuang/internal/graph"
)

type graphQueryStore interface {
	GetPrerequisites(ctx context.Context, questionNumber int) ([]graph.QuestionNode, error)
	GetNextLevelQuestions(ctx context.Context, questionNumber int) ([]graph.QuestionNode, error)
	FindLearningPath(ctx context.Context, startQuestion, endQuestion int) (*graph.LearningPath, error)
	GetRecommendations(ctx context.Context, questionNumber int, limit int) ([]graph.RecommendationResult, error)
	ListQuestions(ctx context.Context) ([]graph.QuestionNode, error)
	ListSkills(ctx context.Context) ([]graph.SkillNode, error)
	ListRelations(ctx context.Context) ([]graph.QuestionRelation, error)
	ListQuestionSkillRelations(ctx context.Context) ([]graph.QuestionSkillRelation, error)
	ListSkillRelations(ctx context.Context) ([]graph.SkillRelation, error)
}

type GraphQueryService struct {
	store graphQueryStore
}

func NewGraphQueryService(store graphQueryStore) *GraphQueryService {
	return &GraphQueryService{store: store}
}

type GraphListStage string

const (
	GraphListStageQuestions              GraphListStage = "questions"
	GraphListStageSkills                 GraphListStage = "skills"
	GraphListStageQuestionRelations      GraphListStage = "question_relations"
	GraphListStageQuestionSkillRelations GraphListStage = "question_skill_relations"
	GraphListStageSkillRelations         GraphListStage = "skill_relations"
)

type GraphQueryError struct {
	Stage GraphListStage
	Err   error
}

func (e *GraphQueryError) Error() string {
	return fmt.Sprintf("graph query failed at stage %s: %v", e.Stage, e.Err)
}

func (e *GraphQueryError) Unwrap() error {
	return e.Err
}

type GraphEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Type   string  `json:"type"`
	Weight float64 `json:"weight"`
}

type GraphListResult struct {
	Questions              []graph.QuestionNode          `json:"questions"`
	Skills                 []graph.SkillNode             `json:"skills"`
	QuestionRelations      []graph.QuestionRelation      `json:"question_relations"`
	QuestionSkillRelations []graph.QuestionSkillRelation `json:"question_skill_relations"`
	SkillRelations         []graph.SkillRelation         `json:"skill_relations"`
	Edges                  []GraphEdge                   `json:"edges"`
	Count                  int                           `json:"count"`
	SkillCount             int                           `json:"skill_count"`
	EdgeCount              int                           `json:"edge_count"`
}

func (s *GraphQueryService) GetPrerequisites(ctx context.Context, questionNumber int) ([]graph.QuestionNode, error) {
	return s.store.GetPrerequisites(ctx, questionNumber)
}

func (s *GraphQueryService) GetNextLevelQuestions(ctx context.Context, questionNumber int) ([]graph.QuestionNode, error) {
	return s.store.GetNextLevelQuestions(ctx, questionNumber)
}

func (s *GraphQueryService) FindLearningPath(ctx context.Context, startQuestion, endQuestion int) (*graph.LearningPath, error) {
	return s.store.FindLearningPath(ctx, startQuestion, endQuestion)
}

func (s *GraphQueryService) GetRecommendations(ctx context.Context, questionNumber int, limit int) ([]graph.RecommendationResult, error) {
	return s.store.GetRecommendations(ctx, questionNumber, limit)
}

func (s *GraphQueryService) ListGraphData(ctx context.Context) (*GraphListResult, error) {
	questions, err := s.store.ListQuestions(ctx)
	if err != nil {
		return nil, &GraphQueryError{Stage: GraphListStageQuestions, Err: err}
	}

	skills, err := s.store.ListSkills(ctx)
	if err != nil {
		return nil, &GraphQueryError{Stage: GraphListStageSkills, Err: err}
	}

	questionRelations, err := s.store.ListRelations(ctx)
	if err != nil {
		return nil, &GraphQueryError{Stage: GraphListStageQuestionRelations, Err: err}
	}

	questionSkillRelations, err := s.store.ListQuestionSkillRelations(ctx)
	if err != nil {
		return nil, &GraphQueryError{Stage: GraphListStageQuestionSkillRelations, Err: err}
	}

	skillRelations, err := s.store.ListSkillRelations(ctx)
	if err != nil {
		return nil, &GraphQueryError{Stage: GraphListStageSkillRelations, Err: err}
	}

	questionIDByNumber := make(map[int]string, len(questions))
	for _, q := range questions {
		if q.QuestionNumber == 0 {
			continue
		}
		questionIDByNumber[q.QuestionNumber] = q.QuestionId
	}

	questionKey := func(questionNumber int) string {
		if qid := questionIDByNumber[questionNumber]; qid != "" {
			return "Q:" + qid
		}
		return "Q:" + strconv.Itoa(questionNumber)
	}

	edges := make([]GraphEdge, 0, len(questionRelations)+len(questionSkillRelations)+len(skillRelations))
	for _, r := range questionRelations {
		edges = append(edges, GraphEdge{
			From:   questionKey(r.FromQuestionNumber),
			To:     questionKey(r.ToQuestionNumber),
			Type:   string(r.RelationType),
			Weight: r.Weight,
		})
	}

	for _, r := range questionSkillRelations {
		edges = append(edges, GraphEdge{
			From:   questionKey(r.QuestionNumber),
			To:     "S:" + r.SkillKey,
			Type:   "HAS_SKILL",
			Weight: r.Weight,
		})
	}

	for _, r := range skillRelations {
		edges = append(edges, GraphEdge{
			From:   "S:" + r.FromKey,
			To:     "S:" + r.ToKey,
			Type:   string(r.RelationType),
			Weight: r.Weight,
		})
	}

	return &GraphListResult{
		Questions:              questions,
		Skills:                 skills,
		QuestionRelations:      questionRelations,
		QuestionSkillRelations: questionSkillRelations,
		SkillRelations:         skillRelations,
		Edges:                  edges,
		Count:                  len(questions),
		SkillCount:             len(skills),
		EdgeCount:              len(edges),
	}, nil
}
