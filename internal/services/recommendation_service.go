package services

import (
	"context"
	"fmt"

	"dachuang/internal/graph"

	"gorm.io/gorm"
)

// RecommendationService 个性化推荐服务
type RecommendationService struct {
	DB           *gorm.DB
	GraphService *graph.QuestionGraphService
}

// NewRecommendationService 创建推荐服务
func NewRecommendationService(db *gorm.DB, graphService *graph.QuestionGraphService) *RecommendationService {
	return &RecommendationService{
		DB:           db,
		GraphService: graphService,
	}
}

// RecommendationItem 推荐项
type RecommendationItem struct {
	QuestionNumber int     `json:"question_number"`
	Title          string  `json:"title"`
	Difficulty     string  `json:"difficulty"`
	Reason         string  `json:"reason"`
	Score          float64 `json:"score"`
	SkillKey       string  `json:"skill_key"`
}

// GetPersonalizedRecommendations 获取个性化题目推荐
func (s *RecommendationService) GetPersonalizedRecommendations(userID string, limit int) ([]RecommendationItem, error) {
	ctx := context.Background()

	if s.GraphService == nil {
		return nil, fmt.Errorf("graph service is not initialized")
	}
	// 1. 获取用户当前的技能掌握情况
	masteries, err := s.GraphService.GetUserMastery(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user mastery: %w", err)
	}

	// 2. 识别“最近发展区” (Zone of Proximal Development)
	// 策略：掌握度在 0.3 - 0.7 之间的技能是最佳学习区
	// 如果所有技能都在 0.8 以上，或者没有技能，则推荐基础题或进阶题

	targetSkills := make([]string, 0)

	for _, m := range masteries {
		if m.Mastery >= 0.2 && m.Mastery < 0.8 {
			targetSkills = append(targetSkills, m.SkillKey)
		}
	}

	// 如果没有找到特定的发展区技能（例如新用户或全满级），则尝试找未解锁的技能
	// 这里的简化逻辑：如果没有靶向技能，则传空列表，让 Graph Service 推荐入门题或基于热门度推荐

	// 3. 查询 Neo4j 获取推荐题目
	recommendations, err := s.GraphService.RecommendQuestionsBySkills(ctx, userID, targetSkills, limit)
	if err != nil {
		return nil, fmt.Errorf("graph recommendation failed: %w", err)
	}

	// 构建 Mastery Map 方便快速查找
	masteryMap := make(map[string]float64)
	for _, m := range masteries {
		masteryMap[m.SkillKey] = m.Mastery
	}

	// 转换结果并生成详细理由
	var result []RecommendationItem
	for _, rec := range recommendations {
		reason := rec.Reason

		// 如果有关联技能，根据掌握度生成更详细的理由
		if rec.SkillKey != "" {
			currentMastery := masteryMap[rec.SkillKey]
			if currentMastery < 0.2 {
				reason = fmt.Sprintf("新技能入门: %s", rec.SkillKey)
			} else if currentMastery < 0.8 {
				reason = fmt.Sprintf("针对性强化: %s (当前: %.2f)", rec.SkillKey, currentMastery)
			} else {
				reason = fmt.Sprintf("高阶挑战: %s (当前: %.2f)", rec.SkillKey, currentMastery)
			}
		}

		result = append(result, RecommendationItem{
			QuestionNumber: rec.QuestionNumber,
			Title:          rec.Title,
			Difficulty:     rec.Difficulty,
			Reason:         reason,
			Score:          rec.Score,
			SkillKey:       rec.SkillKey,
		})
	}

	return result, nil
}
