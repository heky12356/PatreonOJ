package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"dachuang/internal/graph"
	"dachuang/internal/models"

	"gorm.io/gorm"
)

var ErrQuestionNotFound = errors.New("question not found")

type graphAnalysisGraphStore interface {
	CreateRelation(ctx context.Context, relation graph.QuestionRelation) error
	ListSkills(ctx context.Context) ([]graph.SkillNode, error)
	CreateSkillRelation(ctx context.Context, fromKey, toKey string, relationType graph.SkillRelationType, description string) error
}

type graphAnalysisAI interface {
	AnalyzeQuestionRelations(ctx context.Context, newQuestion string, candidateQuestions string) ([]QuestionRelation, error)
	AnalyzeSkillTree(ctx context.Context, skills []string) ([]SkillRelation, error)
}

// GraphAnalysisService 封装图谱 AI 分析相关业务流程。
type GraphAnalysisService struct {
	db           *gorm.DB
	graphService graphAnalysisGraphStore
	aiService    graphAnalysisAI
}

func NewGraphAnalysisService(db *gorm.DB, graphService graphAnalysisGraphStore, aiService graphAnalysisAI) *GraphAnalysisService {
	return &GraphAnalysisService{
		db:           db,
		graphService: graphService,
		aiService:    aiService,
	}
}

func (s *GraphAnalysisService) AnalyzeQuestionRelations(ctx context.Context, questionNumber int) ([]QuestionRelation, int, error) {
	var targetQuestion models.Question
	if err := s.db.Where("question_number = ?", questionNumber).First(&targetQuestion).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrQuestionNotFound
		}
		return nil, 0, fmt.Errorf("query question failed: %w", err)
	}

	candidateQuestions, err := s.findCandidateQuestions(questionNumber, targetQuestion)
	if err != nil {
		return nil, 0, err
	}

	targetJSON, candidatesJSON, err := buildQuestionAnalysisPayload(targetQuestion, candidateQuestions, questionNumber)
	if err != nil {
		return nil, 0, err
	}

	relations, err := s.aiService.AnalyzeQuestionRelations(ctx, targetJSON, candidatesJSON)
	if err != nil {
		return nil, 0, err
	}

	saved := 0
	for _, rel := range relations {
		var relType graph.RelationshipType
		switch rel.RelationType {
		case "PREREQUISITE":
			relType = graph.PREREQUISITE
		case "SIMILAR":
			relType = graph.SIMILAR
		default:
			continue
		}

		graphRel := graph.QuestionRelation{
			FromQuestionNumber: rel.SourceID,
			ToQuestionNumber:   rel.TargetID,
			RelationType:       relType,
			Weight:             1.0,
			Description:        rel.Reason,
			CreatedAt:          time.Now().UTC(),
		}

		if rel.TargetID == 0 {
			graphRel.ToQuestionNumber = questionNumber
		}

		if err := s.graphService.CreateRelation(ctx, graphRel); err == nil {
			saved++
		}
	}

	return relations, saved, nil
}

func (s *GraphAnalysisService) AnalyzeSkillTree(ctx context.Context) ([]SkillRelation, int, int, bool, error) {
	skills, err := s.graphService.ListSkills(ctx)
	if err != nil {
		return nil, 0, 0, false, err
	}

	if len(skills) == 0 {
		return []SkillRelation{}, 0, 0, false, nil
	}

	nameToKey := make(map[string]string, len(skills))
	skillNames := make([]string, 0, len(skills))
	for _, sk := range skills {
		nameToKey[sk.Name] = sk.Key
		skillNames = append(skillNames, sk.Name)
	}

	relations, err := s.aiService.AnalyzeSkillTree(ctx, skillNames)
	if err != nil {
		return nil, 0, 0, true, err
	}

	saved, failed := 0, 0
	for _, rel := range relations {
		fromKey, ok1 := nameToKey[rel.ParentSkill]
		toKey, ok2 := nameToKey[rel.ChildSkill]
		if !ok1 || !ok2 {
			failed++
			continue
		}

		if err := s.graphService.CreateSkillRelation(ctx, fromKey, toKey, graph.SKILL_SUBSUMES, rel.Reason); err != nil {
			log.Printf("创建技能关系失败 %s->%s: %v", fromKey, toKey, err)
			failed++
		} else {
			saved++
		}
	}

	return relations, saved, failed, true, nil
}

func (s *GraphAnalysisService) findCandidateQuestions(questionNumber int, targetQuestion models.Question) ([]models.Question, error) {
	targetTags := strings.Split(targetQuestion.Tags, ",")
	for i := range targetTags {
		targetTags[i] = strings.TrimSpace(targetTags[i])
	}

	tagConditions := make([]string, 0, len(targetTags))
	tagArgs := make([]interface{}, 0, len(targetTags))
	for _, tag := range targetTags {
		if tag != "" {
			tagConditions = append(tagConditions, "tags LIKE ?")
			tagArgs = append(tagArgs, "%"+tag+"%")
		}
	}

	var candidateQuestions []models.Question
	query := s.db.Where("question_number != ?", questionNumber)
	if len(tagConditions) > 0 {
		query = query.Where("("+strings.Join(tagConditions, " OR ")+")", tagArgs...)
	}

	orderClause := fmt.Sprintf("CASE WHEN difficulty = '%s' THEN 0 ELSE 1 END", targetQuestion.Difficulty)
	if err := query.Order(orderClause).Limit(30).Find(&candidateQuestions).Error; err != nil {
		return nil, fmt.Errorf("query candidate questions failed: %w", err)
	}

	if len(candidateQuestions) < 20 {
		var extraQuestions []models.Question
		existingIDs := make([]int, len(candidateQuestions)+1)
		existingIDs[0] = questionNumber
		for i, q := range candidateQuestions {
			existingIDs[i+1] = q.QuestionNumber
		}

		if err := s.db.Where("question_number NOT IN ?", existingIDs).
			Limit(20 - len(candidateQuestions)).
			Find(&extraQuestions).Error; err != nil {
			return nil, fmt.Errorf("query extra questions failed: %w", err)
		}
		candidateQuestions = append(candidateQuestions, extraQuestions...)
	}

	return candidateQuestions, nil
}

func buildQuestionAnalysisPayload(targetQuestion models.Question, candidateQuestions []models.Question, currentQuestionNumber int) (string, string, error) {
	targetJSONBytes, err := json.Marshal(map[string]interface{}{
		"id":         targetQuestion.QuestionNumber,
		"title":      targetQuestion.Title,
		"tags":       targetQuestion.Tags,
		"difficulty": targetQuestion.Difficulty,
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal target question failed: %w", err)
	}

	candidates := make([]map[string]interface{}, 0, len(candidateQuestions))
	for _, q := range candidateQuestions {
		if q.QuestionNumber == currentQuestionNumber {
			continue
		}
		candidates = append(candidates, map[string]interface{}{
			"id":         q.QuestionNumber,
			"title":      q.Title,
			"tags":       q.Tags,
			"difficulty": q.Difficulty,
		})
	}

	candidatesJSONBytes, err := json.Marshal(candidates)
	if err != nil {
		return "", "", fmt.Errorf("marshal candidates failed: %w", err)
	}

	return string(targetJSONBytes), string(candidatesJSONBytes), nil
}
