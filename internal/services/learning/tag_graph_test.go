package learning

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"patreonoj/internal/models"
	"testing"
)

func TestTagGraphsAreExactPublishedAndPaged(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	if err := db.AutoMigrate(&models.Question{}, &models.QuestionRelationEdge{}); err != nil {
		t.Fatal(err)
	}
	questions := []models.Question{{QuestionId: "A", Title: "long title", Tags: "数组,前缀和", Status: "published"}, {QuestionId: "B", Title: "second", Tags: "数组", Status: "published"}, {QuestionId: "C", Tags: "二维数组", Status: "published"}, {QuestionId: "D", Tags: "数组", Status: "draft"}, {QuestionId: "duplicate", Tags: "数组", Status: "published"}, {QuestionId: "duplicate", Tags: "数组", Status: "draft"}}
	if err := db.Create(&questions).Error; err != nil {
		t.Fatal(err)
	}
	for _, edge := range []models.QuestionRelationEdge{{FromQuestionID: "A", ToQuestionID: "B", EdgeType: models.QuestionRelationEdgeTypeSimilar}, {FromQuestionID: "A", ToQuestionID: "C", EdgeType: models.QuestionRelationEdgeTypeNextLevel}} {
		if err := db.Create(&edge).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := NewLearningService(db)
	catalog, err := s.TagCatalog()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, item := range catalog {
		counts[item.Name] = item.QuestionCount
	}
	if counts["数组"] != 2 || counts["二维数组"] != 1 {
		t.Fatalf("catalog: %+v", catalog)
	}
	graph, err := s.BuildTagGraph("数组", "", 1, 60)
	if err != nil || graph.TotalNodes != 2 || len(graph.Nodes) != 2 || len(graph.Edges) != 1 || graph.Edges[0].ToQuestionID != "B" {
		t.Fatalf("graph leaked other tags/drafts: %+v %v", graph, err)
	}
	paged, err := s.BuildTagGraph("数组", "", 2, 1)
	if err != nil || len(paged.Nodes) != 1 || paged.Nodes[0].QuestionID != "B" || !paged.Truncated {
		t.Fatalf("pagination: %+v %v", paged, err)
	}
	filtered, err := s.BuildTagGraph("数组", "long title", 1, 60)
	if err != nil || len(filtered.Nodes) != 1 || filtered.TagQuestionCount != 2 || filtered.TotalNodes != 1 {
		t.Fatalf("title filter: %+v %v", filtered, err)
	}
}
