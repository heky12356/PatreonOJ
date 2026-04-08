package graph

import (
	"fmt"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRelationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.QuestionRelationEdge{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestRelationService_List(t *testing.T) {
	db := newRelationServiceTestDB(t)
	if err := db.Create(&models.QuestionRelationEdge{
		FromQuestionID: "q-1",
		ToQuestionID:   "q-2",
		EdgeType:       models.QuestionRelationEdgeTypePrerequisite,
	}).Error; err != nil {
		t.Fatalf("seed relation failed: %v", err)
	}

	svc := NewRelationService(db)
	list, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(list))
	}
	if list[0].EdgeType != models.QuestionRelationEdgeTypePrerequisite {
		t.Fatalf("unexpected relation value: %s", list[0].EdgeType)
	}
}
