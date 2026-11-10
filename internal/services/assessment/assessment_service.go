package assessment

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"time"

	"patreonoj/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const masteryLearningRate = 0.2

// AssessmentService updates user skill mastery from judging results.
type AssessmentService struct {
	DB *gorm.DB
}

func NewAssessmentService(db *gorm.DB) *AssessmentService {
	return &AssessmentService{DB: db}
}

// UpdateUserMasteryBasedOnResult updates skill mastery after an accepted submission.
func (s *AssessmentService) UpdateUserMasteryBasedOnResult(userID string, questionID int, isCorrect bool) error {
	userID = strings.TrimSpace(userID)
	if !isCorrect || userID == "" || questionID <= 0 {
		return nil
	}
	if s == nil || s.DB == nil {
		return errors.New("assessment database is not initialized")
	}

	var question models.Question
	if err := s.DB.Where("id = ?", questionID).First(&question).Error; err != nil {
		return fmt.Errorf("load question for mastery update: %w", err)
	}

	skills := parseSkillKeys(question.Tags)
	if len(skills) == 0 {
		return nil
	}

	factor := difficultyFactor(question.Difficulty)
	return s.DB.Transaction(func(tx *gorm.DB) error {
		for _, skill := range skills {
			if err := updateSkillMastery(tx, userID, skill, factor); err != nil {
				return err
			}
		}
		return nil
	})
}

func updateSkillMastery(tx *gorm.DB, userID string, skillKey string, difficultyFactor float64) error {
	factor := clamp01(masteryLearningRate * maxFloat(difficultyFactor, 0))
	mastery := models.UserSkillMastery{UserID: userID, SkillKey: skillKey, Mastery: factor}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "skill_key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"mastery": gorm.Expr("CASE WHEN mastery < 0 THEN ? WHEN mastery > 1 THEN 1 ELSE mastery + ? * (1 - mastery) END", factor, factor),
			"updated_at": time.Now(),
		}),
	}).Create(&mastery).Error
}

func nextMastery(current float64, difficultyFactor float64) float64 {
	current = clamp01(current)
	difficultyFactor = maxFloat(difficultyFactor, 0)
	next := current + masteryLearningRate*(1-current)*difficultyFactor
	return clamp01(next)
}

func parseSkillKeys(tags string) []string {
	fields := strings.FieldsFunc(tags, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '|', '/', '\\':
			return true
		default:
			return unicode.IsSpace(r)
		}
	})

	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		key := normalizeSkillKey(field)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func normalizeSkillKey(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\u3000", " ")
	name = strings.Join(strings.Fields(name), " ")
	return strings.ToLower(name)
}

func difficultyFactor(difficulty string) float64 {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "简单", "easy", "simple":
		return 0.5
	case "中等", "medium":
		return 0.8
	case "困难", "hard":
		return 1.2
	default:
		return 0.6
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
