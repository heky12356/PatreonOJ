package assessment

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAssessmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Question{}, &models.UserSkillMastery{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedAssessmentQuestion(t *testing.T, db *gorm.DB, question models.Question) models.Question {
	t.Helper()
	if err := db.Create(&question).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
	return question
}

func TestAssessmentService_UpdateUserMasteryBasedOnResultCreatesMasteries(t *testing.T) {
	db := newAssessmentTestDB(t)
	question := seedAssessmentQuestion(t, db, models.Question{
		QuestionId:  "P1001",
		Title:       "prefix sum",
		Difficulty:  "中等",
		Tags:        "Array, 前缀和，DP ; array",
		Status:      "published",
		TimeLimit:   1000,
		MemoryLimit: 128,
	})

	svc := NewAssessmentService(db)
	if err := svc.UpdateUserMasteryBasedOnResult("u1", question.Id, true); err != nil {
		t.Fatalf("UpdateUserMasteryBasedOnResult failed: %v", err)
	}

	var rows []models.UserSkillMastery
	if err := db.Order("skill_key").Find(&rows).Error; err != nil {
		t.Fatalf("query mastery rows failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 unique skills, got %d: %+v", len(rows), rows)
	}

	wantSkills := []string{"array", "dp", "前缀和"}
	for i, row := range rows {
		if row.UserID != "u1" {
			t.Fatalf("unexpected user_id: %s", row.UserID)
		}
		if row.SkillKey != wantSkills[i] {
			t.Fatalf("skill[%d] got %q want %q", i, row.SkillKey, wantSkills[i])
		}
		if !almostEqual(row.Mastery, 0.16) {
			t.Fatalf("skill %s mastery got %.4f want 0.16", row.SkillKey, row.Mastery)
		}
	}
}

func TestAssessmentService_UpdateUserMasteryBasedOnResultIncreasesAndCaps(t *testing.T) {
	db := newAssessmentTestDB(t)
	question := seedAssessmentQuestion(t, db, models.Question{
		QuestionId: "P1002",
		Title:      "hard graph",
		Difficulty: "Hard",
		Tags:       "graph",
		Status:     "published",
	})
	if err := db.Create(&models.UserSkillMastery{
		UserID:   "u2",
		SkillKey: "graph",
		Mastery:  0.95,
	}).Error; err != nil {
		t.Fatalf("seed mastery failed: %v", err)
	}

	svc := NewAssessmentService(db)
	if err := svc.UpdateUserMasteryBasedOnResult("u2", question.Id, true); err != nil {
		t.Fatalf("UpdateUserMasteryBasedOnResult failed: %v", err)
	}

	var row models.UserSkillMastery
	if err := db.Where("user_id = ? AND skill_key = ?", "u2", "graph").First(&row).Error; err != nil {
		t.Fatalf("query mastery failed: %v", err)
	}
	if row.Mastery <= 0.95 {
		t.Fatalf("expected mastery to increase, got %.4f", row.Mastery)
	}
	if row.Mastery > 1 {
		t.Fatalf("expected mastery capped at 1, got %.4f", row.Mastery)
	}
}

func TestAssessmentService_UpdateUserMasteryBasedOnResultSkipsIncorrect(t *testing.T) {
	db := newAssessmentTestDB(t)
	question := seedAssessmentQuestion(t, db, models.Question{
		QuestionId: "P1003",
		Title:      "array",
		Difficulty: "Easy",
		Tags:       "array",
		Status:     "published",
	})

	svc := NewAssessmentService(db)
	if err := svc.UpdateUserMasteryBasedOnResult("u3", question.Id, false); err != nil {
		t.Fatalf("UpdateUserMasteryBasedOnResult failed: %v", err)
	}

	var count int64
	if err := db.Model(&models.UserSkillMastery{}).Count(&count).Error; err != nil {
		t.Fatalf("count mastery failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no mastery rows for incorrect result, got %d", count)
	}
}

func TestAssessmentService_UpdateUserMasteryBasedOnResultSkipsEmptyTags(t *testing.T) {
	db := newAssessmentTestDB(t)
	question := seedAssessmentQuestion(t, db, models.Question{
		QuestionId: "P1004",
		Title:      "untagged",
		Difficulty: "Easy",
		Status:     "published",
	})

	svc := NewAssessmentService(db)
	if err := svc.UpdateUserMasteryBasedOnResult("u4", question.Id, true); err != nil {
		t.Fatalf("UpdateUserMasteryBasedOnResult failed: %v", err)
	}

	var count int64
	if err := db.Model(&models.UserSkillMastery{}).Count(&count).Error; err != nil {
		t.Fatalf("count mastery failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no mastery rows for empty tags, got %d", count)
	}
}

func almostEqual(a float64, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
