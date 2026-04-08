package learning

import (
	"fmt"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLearningServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Question{},
		&models.Submission{},
		&models.UserSolvedQuestion{},
		&models.QuestionRelationEdge{},
	); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedLearningUser(t *testing.T, db *gorm.DB, uuid string) {
	t.Helper()
	user := models.User{
		Username: "u_" + uuid,
		Password: "pwd",
		UUID:     uuid,
		Status:   "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
}

func seedLearningQuestion(t *testing.T, db *gorm.DB, q models.Question) {
	t.Helper()
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
}

func TestLearningService_MasteredRecommendNextAndSimilar(t *testing.T) {
	db := newLearningServiceTestDB(t)
	seedLearningUser(t, db, "u1")

	q1 := models.Question{QuestionId: "q1", Title: "q1", Difficulty: "Easy", Tags: "array,hash", Status: "published"}
	q2 := models.Question{QuestionId: "q2", Title: "q2", Difficulty: "Medium", Tags: "array", Status: "published"}
	q3 := models.Question{QuestionId: "q3", Title: "q3", Difficulty: "Medium", Tags: "hash", Status: "published"}
	seedLearningQuestion(t, db, q1)
	seedLearningQuestion(t, db, q2)
	seedLearningQuestion(t, db, q3)

	if err := db.Where("question_id = ?", "q1").First(&q1).Error; err != nil {
		t.Fatalf("load q1 failed: %v", err)
	}

	if err := db.Create(&models.Submission{
		ID:         "s1",
		UserID:     "u1",
		QuestionID: q1.Id,
		Status:     "completed",
	}).Error; err != nil {
		t.Fatalf("seed submission failed: %v", err)
	}
	if err := db.Create(&models.Submission{
		ID:         "s2",
		UserID:     "u1",
		QuestionID: q1.Id,
		Status:     "completed",
	}).Error; err != nil {
		t.Fatalf("seed submission failed: %v", err)
	}

	if err := db.Create(&models.UserSolvedQuestion{
		UserUUID:   "u1",
		QuestionID: q1.Id,
	}).Error; err != nil {
		t.Fatalf("seed solved failed: %v", err)
	}

	if err := db.Create(&models.QuestionRelationEdge{
		FromQuestionID: "q1",
		ToQuestionID:   "q2",
		EdgeType:       models.QuestionRelationEdgeTypeNextLevel,
	}).Error; err != nil {
		t.Fatalf("seed edge failed: %v", err)
	}
	if err := db.Create(&models.QuestionRelationEdge{
		FromQuestionID: "q1",
		ToQuestionID:   "q3",
		EdgeType:       models.QuestionRelationEdgeTypeSimilar,
	}).Error; err != nil {
		t.Fatalf("seed edge failed: %v", err)
	}

	svc := NewLearningService(db)
	got, err := svc.RecommendNextQuestions("u1", q1.Id, 5)
	if err != nil {
		t.Fatalf("RecommendNextQuestions failed: %v", err)
	}

	if got.State != LearningStateMastered {
		t.Fatalf("expected mastered, got %s", got.State)
	}
	if got.Attempts != 2 || got.PassCount != 1 {
		t.Fatalf("unexpected attempts/pass_count: %d/%d", got.Attempts, got.PassCount)
	}
	if len(got.Recommendations) == 0 {
		t.Fatalf("expected recommendations")
	}
}

func TestLearningService_ChallengedRecommendPrerequisite(t *testing.T) {
	db := newLearningServiceTestDB(t)
	seedLearningUser(t, db, "u2")

	base := models.Question{QuestionId: "base", Title: "base", Difficulty: "Easy", Tags: "array", Status: "published"}
	hard := models.Question{QuestionId: "hard", Title: "hard", Difficulty: "Hard", Tags: "array,dp", Status: "published"}
	seedLearningQuestion(t, db, base)
	seedLearningQuestion(t, db, hard)

	if err := db.Where("question_id = ?", "hard").First(&hard).Error; err != nil {
		t.Fatalf("load hard failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := db.Create(&models.Submission{
			ID:         fmt.Sprintf("s_ch_%d", i),
			UserID:     "u2",
			QuestionID: hard.Id,
			Status:     "error",
		}).Error; err != nil {
			t.Fatalf("seed submission failed: %v", err)
		}
	}

	if err := db.Create(&models.QuestionRelationEdge{
		FromQuestionID: "base",
		ToQuestionID:   "hard",
		EdgeType:       models.QuestionRelationEdgeTypePrerequisite,
	}).Error; err != nil {
		t.Fatalf("seed edge failed: %v", err)
	}

	svc := NewLearningService(db)
	got, err := svc.RecommendNextQuestions("u2", hard.Id, 3)
	if err != nil {
		t.Fatalf("RecommendNextQuestions failed: %v", err)
	}

	if got.State != LearningStateChallenged {
		t.Fatalf("expected challenged, got %s", got.State)
	}
	if len(got.Recommendations) == 0 {
		t.Fatalf("expected recommendations")
	}
	if got.Recommendations[0].QuestionID != "base" {
		t.Fatalf("expected prerequisite 'base', got %s", got.Recommendations[0].QuestionID)
	}
}
