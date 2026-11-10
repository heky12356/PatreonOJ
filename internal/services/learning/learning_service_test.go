package learning

import (
	"errors"
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
		&models.UserSkillMastery{},
	); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestDenseQuestionGraphAvoidsSQLVariableLimit(t *testing.T) {
	db := newLearningServiceTestDB(t)
	questions := make([]models.Question, 130)
	for i := range questions {
		questions[i] = models.Question{QuestionId: fmt.Sprintf("dense-%d", i), Title: "Dense", Tags: "array", Status: "published"}
	}
	if err := db.CreateInBatches(questions, 100).Error; err != nil {
		t.Fatal(err)
	}
	edges := make([]models.QuestionRelationEdge, 0, 130*129)
	for i := range questions {
		for j := range questions {
			if i != j {
				edges = append(edges, models.QuestionRelationEdge{FromQuestionID: questions[i].QuestionId, ToQuestionID: questions[j].QuestionId, EdgeType: models.QuestionRelationEdgeTypeSimilar})
			}
		}
	}
	if err := db.CreateInBatches(edges, 500).Error; err != nil {
		t.Fatal(err)
	}
	graph, err := NewLearningService(db).BuildQuestionGraph(questions[0].Id, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 130 || len(graph.Edges) != 130*129 || graph.Truncated {
		t.Fatalf("incomplete graph: nodes=%d edges=%d truncated=%v", len(graph.Nodes), len(graph.Edges), graph.Truncated)
	}
}

func TestQuestionGraphBoundsNodeCount(t *testing.T) {
	db := newLearningServiceTestDB(t)
	questions := make([]models.Question, 270)
	for i := range questions {
		questions[i] = models.Question{QuestionId: fmt.Sprintf("star-%d", i), Title: "Star", Status: "published"}
	}
	if err := db.CreateInBatches(questions, 100).Error; err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(questions); i++ {
		if err := db.Create(&models.QuestionRelationEdge{FromQuestionID: questions[0].QuestionId, ToQuestionID: questions[i].QuestionId, EdgeType: models.QuestionRelationEdgeTypeSimilar}).Error; err != nil {
			t.Fatal(err)
		}
	}
	graph, err := NewLearningService(db).BuildQuestionGraph(questions[0].Id, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) > 256 || !graph.Truncated {
		t.Fatalf("graph not bounded: %d truncated=%v", len(graph.Nodes), graph.Truncated)
	}
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
	if got.Recommendations[0].Reason == "" {
		t.Fatalf("expected non-empty reason")
	}
	if strings.Contains(strings.ToLower(got.Recommendations[0].Reason), "practice") {
		t.Fatalf("expected chinese reason, got %q", got.Recommendations[0].Reason)
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
	if got.Recommendations[0].Reason == "" {
		t.Fatalf("expected non-empty reason")
	}
	if strings.Contains(strings.ToLower(got.Recommendations[0].Reason), "prerequisite") {
		t.Fatalf("expected chinese reason, got %q", got.Recommendations[0].Reason)
	}
}

func TestLearningService_BuildQuestionGraph(t *testing.T) {
	db := newLearningServiceTestDB(t)

	q1 := models.Question{QuestionId: "P1001", Title: "A", Difficulty: "Easy", Tags: "array", Status: "published"}
	q2 := models.Question{QuestionId: "P1002", Title: "B", Difficulty: "Medium", Tags: "array,dp", Status: "published"}
	q3 := models.Question{QuestionId: "P1003", Title: "C", Difficulty: "Hard", Tags: "dp", Status: "published"}
	seedLearningQuestion(t, db, q1)
	seedLearningQuestion(t, db, q2)
	seedLearningQuestion(t, db, q3)

	if err := db.Create(&models.QuestionRelationEdge{
		FromQuestionID: "P1001",
		ToQuestionID:   "P1002",
		EdgeType:       models.QuestionRelationEdgeTypeNextLevel,
	}).Error; err != nil {
		t.Fatalf("seed edge failed: %v", err)
	}
	if err := db.Create(&models.QuestionRelationEdge{
		FromQuestionID: "P1002",
		ToQuestionID:   "P1003",
		EdgeType:       models.QuestionRelationEdgeTypePrerequisite,
	}).Error; err != nil {
		t.Fatalf("seed edge failed: %v", err)
	}

	if err := db.Where("question_id = ?", "P1001").First(&q1).Error; err != nil {
		t.Fatalf("load q1 failed: %v", err)
	}

	svc := NewLearningService(db)
	graph, err := svc.BuildQuestionGraph(q1.Id, 3)
	if err != nil {
		t.Fatalf("BuildQuestionGraph failed: %v", err)
	}
	if graph.RootQuestionID != "P1001" || graph.RootQuestionPK != q1.Id {
		t.Fatalf("unexpected root: %+v", graph)
	}
	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(graph.Edges))
	}
	if graph.Tree == nil || graph.Tree.QuestionID != "P1001" {
		t.Fatalf("unexpected tree root: %+v", graph.Tree)
	}
	if len(graph.Tree.Children) == 0 {
		t.Fatalf("expected tree children")
	}
}

func TestLearningService_BuildUserFeedback(t *testing.T) {
	db := newLearningServiceTestDB(t)
	seedLearningUser(t, db, "u-feedback")

	solvedDP := models.Question{QuestionId: "P9001", Title: "Solved DP", Difficulty: "Easy", Tags: "dp", Status: "published"}
	weakDP := models.Question{QuestionId: "P9002", Title: "Weak DP", Difficulty: "Medium", Tags: "dp,array", Status: "published"}
	arrayOnly := models.Question{QuestionId: "P9003", Title: "Array", Difficulty: "Easy", Tags: "array", Status: "published"}
	if err := db.Create(&solvedDP).Error; err != nil {
		t.Fatalf("seed solvedDP failed: %v", err)
	}
	if err := db.Create(&weakDP).Error; err != nil {
		t.Fatalf("seed weakDP failed: %v", err)
	}
	if err := db.Create(&arrayOnly).Error; err != nil {
		t.Fatalf("seed arrayOnly failed: %v", err)
	}
	if err := db.Create(&models.UserSolvedQuestion{
		UserUUID:   "u-feedback",
		QuestionID: solvedDP.Id,
	}).Error; err != nil {
		t.Fatalf("seed solved record failed: %v", err)
	}
	if err := db.Create(&[]models.UserSkillMastery{
		{UserID: "u-feedback", SkillKey: "dp", Mastery: 0.2},
		{UserID: "u-feedback", SkillKey: "array", Mastery: 0.9},
		{UserID: "u-feedback", SkillKey: "graph", Mastery: 0.55},
	}).Error; err != nil {
		t.Fatalf("seed masteries failed: %v", err)
	}

	svc := NewLearningService(db)
	got, err := svc.BuildUserFeedback("u-feedback", 3)
	if err != nil {
		t.Fatalf("BuildUserFeedback failed: %v", err)
	}

	if got.UserUUID != "u-feedback" {
		t.Fatalf("unexpected user uuid: %s", got.UserUUID)
	}
	if len(got.WeakSkills) != 1 || got.WeakSkills[0] != "dp" {
		t.Fatalf("unexpected weak skills: %+v", got.WeakSkills)
	}
	if len(got.MasteredSkills) != 1 || got.MasteredSkills[0] != "array" {
		t.Fatalf("unexpected mastered skills: %+v", got.MasteredSkills)
	}
	if got.Stage == "" || got.PositiveFeedback == "" {
		t.Fatalf("expected non-empty stage and feedback: %+v", got)
	}
	if len(got.NextQuestions) == 0 {
		t.Fatalf("expected next questions")
	}
	for _, q := range got.NextQuestions {
		if q.QuestionID == solvedDP.QuestionId {
			t.Fatalf("expected solved question to be excluded")
		}
	}
	if got.NextQuestions[0].QuestionID != weakDP.QuestionId {
		t.Fatalf("expected weak dp question first, got %+v", got.NextQuestions[0])
	}
	if !strings.Contains(got.NextQuestions[0].Reason, "dp") {
		t.Fatalf("expected reason to mention weak skill, got %q", got.NextQuestions[0].Reason)
	}
}

func TestLearningService_BuildUserFeedbackFallbackWithoutMastery(t *testing.T) {
	db := newLearningServiceTestDB(t)
	seedLearningUser(t, db, "u-new")
	seedLearningQuestion(t, db, models.Question{QuestionId: "P9101", Title: "Start", Difficulty: "Easy", Tags: "array", Status: "published"})

	svc := NewLearningService(db)
	got, err := svc.BuildUserFeedback("u-new", 5)
	if err != nil {
		t.Fatalf("BuildUserFeedback failed: %v", err)
	}

	if got.Stage != "起步训练阶段" {
		t.Fatalf("unexpected stage: %s", got.Stage)
	}
	if len(got.WeakSkills) != 0 || len(got.MasteredSkills) != 0 {
		t.Fatalf("expected empty skills, got weak=%+v mastered=%+v", got.WeakSkills, got.MasteredSkills)
	}
	if len(got.NextQuestions) != 1 || got.NextQuestions[0].QuestionID != "P9101" {
		t.Fatalf("unexpected fallback questions: %+v", got.NextQuestions)
	}
}

func TestLearningService_BuildUserFeedbackUserNotFound(t *testing.T) {
	db := newLearningServiceTestDB(t)
	svc := NewLearningService(db)

	_, err := svc.BuildUserFeedback("missing", 5)
	if !errors.Is(err, ErrLearningUserNotFound) {
		t.Fatalf("expected ErrLearningUserNotFound, got %v", err)
	}
}
