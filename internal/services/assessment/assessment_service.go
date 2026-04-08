package assessment

import (
	"context"
	"fmt"
	"strings"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

// AssessmentService updates user mastery in MySQL.
type AssessmentService struct {
	DB *gorm.DB
}

func NewAssessmentService(db *gorm.DB) *AssessmentService {
	return &AssessmentService{DB: db}
}

func normalizeSkillKey(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\u3000", " ")
	name = strings.Join(strings.Fields(name), " ")
	return strings.ToLower(name)
}

// UpdateUserMasteryBasedOnResult updates mastery from AC submissions.
func (s *AssessmentService) UpdateUserMasteryBasedOnResult(userId string, questionId int, isCorrect bool) error {
	var question models.Question
	if err := s.DB.Where("id = ?", questionId).First(&question).Error; err != nil {
		return fmt.Errorf("question not found: %w", err)
	}

	skills := strings.Split(question.Tags, ",")

	for _, skillKey := range skills {
		skillKey = normalizeSkillKey(skillKey)
		if skillKey == "" {
			continue
		}

		currentMastery, err := s.getCurrentMastery(userId, skillKey)
		if err != nil {
			continue
		}

		if isCorrect {
			diffFactor := getDifficultyFactor(question.Difficulty)
			learningRate := 0.2
			delta := learningRate * (1.0 - currentMastery) * diffFactor
			newMastery := currentMastery + delta
			if newMastery > 1.0 {
				newMastery = 1.0
			}

			if err := s.updateMastery(context.Background(), userId, skillKey, newMastery); err != nil {
				fmt.Printf("update mastery failed: %v\n", err)
			}
		}
	}

	return nil
}

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

func (s *AssessmentService) updateMastery(_ context.Context, userId string, skillKey string, mastery float64) error {
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
	return nil
}

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
