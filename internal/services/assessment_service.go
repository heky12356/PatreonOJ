package services

import (
	"context"
	"fmt"
	"strings"

	"dachuang/internal/graph"
	"dachuang/internal/models"

	"gorm.io/gorm"
)

// AssessmentService 能力评估服务
type AssessmentService struct {
	DB           *gorm.DB
	GraphService *graph.QuestionGraphService
}

// NewAssessmentService 创建能力评估服务
func NewAssessmentService(db *gorm.DB, graphService *graph.QuestionGraphService) *AssessmentService {
	return &AssessmentService{
		DB:           db,
		GraphService: graphService,
	}
}

// normalizeSkillKey 标准化技能键（小写、去空格、替换全角空格）
func normalizeSkillKey(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "　", " ")
	name = strings.Join(strings.Fields(name), " ")
	return strings.ToLower(name)
}

// UpdateUserMasteryBasedOnResult 根据用户做题结果更新技能掌握度
func (s *AssessmentService) UpdateUserMasteryBasedOnResult(userId string, questionId int, isCorrect bool) error {
	// 1. 获取题目信息（包括难度和关联技能）
	// 从数据库获取题目的 Tags 字段作为技能

	ctx := context.Background()

	// 从数据库获取题目基本信息（难度）
	var question models.Question
	// 这边submission中存的question_id就是question表中的主键id
	if err := s.DB.Where("id = ?", questionId).First(&question).Error; err != nil {
		return fmt.Errorf("找不到题目: %w", err)
	}

	// 解析题目 Tags 字段作为技能 Key
	skills := strings.Split(question.Tags, ",")

	// 2. 对于每个技能，计算新的掌握度
	for _, skillKey := range skills {
		skillKey = normalizeSkillKey(skillKey)
		if skillKey == "" {
			continue
		}
		// 获取当前掌握度
		currentMastery, err := s.getCurrentMastery(userId, skillKey)
		if err != nil {
			continue
		}

		// 计算增量
		// 算法：
		// 如果做对了 (AC): 掌握度增加。增加幅度取决于题目难度和当前掌握度。
		//     New = Old + LearningRate * (1 - Old) * DifficultyFactor
		// 如果做错了: 掌握度可能微调或不变（暂时设计为不变）
		// 此处仅实现 AC 后的增长

		if isCorrect {
			diffFactor := getDifficultyFactor(question.Difficulty)
			learningRate := 0.2 // 基础学习率

			// 核心公式: 随着掌握度提高，增长变慢；题目越难，增长越快
			delta := learningRate * (1.0 - currentMastery) * diffFactor
			newMastery := currentMastery + delta
			if newMastery > 1.0 {
				newMastery = 1.0
			}

			// 3. 更新数据库和图谱
			if err := s.updateMastery(ctx, userId, skillKey, newMastery); err != nil {
				fmt.Printf("更新掌握度失败: %v\n", err)
			}
		}
	}

	return nil
}

// 获取当前掌握度
func (s *AssessmentService) getCurrentMastery(userId string, skillKey string) (float64, error) {
	var record models.UserSkillMastery
	err := s.DB.Where("user_id = ? AND skill_key = ?", userId, skillKey).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0.0, nil
		}
		return 0.0, err
	}
	return record.Mastery, nil
}

// 更新掌握度
func (s *AssessmentService) updateMastery(ctx context.Context, userId string, skillKey string, mastery float64) error {
	// 更新sql数据库
	var record models.UserSkillMastery
	err := s.DB.Where("user_id = ? AND skill_key = ?", userId, skillKey).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		record = models.UserSkillMastery{
			UserID:   userId,
			SkillKey: skillKey,
			Mastery:  mastery,
		}
		if err := s.DB.Create(&record).Error; err != nil {
			return err
		}
	} else {
		record.Mastery = mastery
		if err := s.DB.Save(&record).Error; err != nil {
			return err
		}
	}

	// 更新图
	if s.GraphService == nil {
		return nil
	}
	return s.GraphService.UpdateUserMastery(ctx, userId, skillKey, mastery)
}

// 获取难度对应的
func getDifficultyFactor(diff string) float64 {
	switch diff {
	case "简单", "Easy":
		return 0.5
	case "中等", "Medium":
		return 0.8
	case "困难", "Hard":
		return 1.2
	default:
		return 0.5
	}
}
