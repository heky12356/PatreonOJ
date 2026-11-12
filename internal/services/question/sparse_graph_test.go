package question

import (
	"encoding/json"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"patreonoj/internal/models"
	"patreonoj/internal/services/learning"
	"testing"
	"time"
)

func sparseDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Question{}, &models.QuestionRelationEdge{}); err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	t.Cleanup(func() { sql.Close() })
	return db
}

func TestSparseGraph500QuestionsHasLinearEdgeBudget(t *testing.T) {
	db := sparseDB(t)
	qs := make([]models.Question, 500)
	for i := range qs {
		qs[i] = models.Question{QuestionId: fmt.Sprintf("PUB_TEST_%04d", i), Title: fmt.Sprintf("Problem %d", i), Tags: "数组,前缀和", Difficulty: "简单", Status: "published"}
	}
	if err := db.CreateInBatches(qs, 100).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	report, err := RebuildAutomaticGraph(db, nil, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Automatic > 500*8 {
		t.Fatalf("quadratic growth: %d", report.Automatic)
	}
	var worst struct{ N int }
	if err := db.Model(&models.QuestionRelationEdge{}).Select("COUNT(*) AS n").Where("edge_type = ? AND source = ?", models.QuestionRelationEdgeTypeSimilar, "auto").Group("from_question_id").Order("n DESC").Limit(1).Scan(&worst).Error; err != nil {
		t.Fatal(err)
	}
	if worst.N > 8 {
		t.Fatalf("similar hub degree %d", worst.N)
	}
	buildTime := time.Since(started)
	started = time.Now()
	graph, err := learning.NewLearningService(db).BuildQuestionGraphLimited(qs[250].Id, 3, 120, 1200)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(graph)
	if len(graph.Nodes) > 120 || len(graph.Edges) > 1200 {
		t.Fatal("unbounded response")
	}
	t.Logf("500 questions: %d auto edges, rebuild=%s, cold graph=%s, nodes=%d edges=%d bytes=%d", report.Automatic, buildTime, time.Since(started), len(graph.Nodes), len(graph.Edges), len(encoded))
}

func TestRebuildPreservesManualEdgesAndPreviewRollsBack(t *testing.T) {
	db := sparseDB(t)
	qs := []models.Question{{QuestionId: "A", Tags: "数组", Status: "published"}, {QuestionId: "PUB_B", Tags: "数组", Status: "published"}, {QuestionId: "C", Tags: "图论", Status: "published"}}
	if err := db.Create(&qs).Error; err != nil {
		t.Fatal(err)
	}
	manual := models.QuestionRelationEdge{FromQuestionID: "C", ToQuestionID: "A", EdgeType: models.QuestionRelationEdgeTypeNextLevel, Source: "manual"}
	legacy := models.QuestionRelationEdge{FromQuestionID: "A", ToQuestionID: "PUB_B", EdgeType: models.QuestionRelationEdgeTypeSimilar}
	for _, edge := range []models.QuestionRelationEdge{manual, legacy} {
		if err := db.Create(&edge).Error; err != nil {
			t.Fatal(err)
		}
	}
	missing := models.QuestionRelationEdge{FromQuestionID: "MISSING", ToQuestionID: "A", EdgeType: models.QuestionRelationEdgeTypeNextLevel}
	if _, err := RebuildAutomaticGraph(db, []models.QuestionRelationEdge{missing}, "PUB_", true); err == nil {
		t.Fatal("missing protected relationship did not abort")
	}
	if _, err := RebuildAutomaticGraph(db, []models.QuestionRelationEdge{manual}, "PUB_", false); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&models.QuestionRelationEdge{}).Count(&count)
	if count != 2 {
		t.Fatal("preview changed rows")
	}
	report, err := RebuildAutomaticGraph(db, []models.QuestionRelationEdge{manual}, "PUB_", true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Classified != 1 || report.Protected != 1 {
		t.Fatalf("unexpected report %+v", report)
	}
	var found models.QuestionRelationEdge
	if err := db.Where("from_question_id = ? AND to_question_id = ? AND source = ?", "C", "A", "manual").First(&found).Error; err != nil {
		t.Fatal("manual edge lost")
	}
	if err := db.Model(&qs[0]).Update("tags", "字符串").Error; err != nil {
		t.Fatal(err)
	}
	if err := refreshQuestionEdges(db, qs[0].Id); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", found.ID).First(&found).Error; err != nil {
		t.Fatal("refresh deleted manual relation")
	}
}
