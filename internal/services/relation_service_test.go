package services

import (
	"fmt"
	"strings"
	"testing"

	"dachuang/internal/models"

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
	if err := db.AutoMigrate(&models.Relation{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestRelationService_List(t *testing.T) {
	db := newRelationServiceTestDB(t)
	if err := db.Create(&models.Relation{
		SourceID: 1,
		TargetID: 2,
		Relation: "depends_on",
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
	if list[0].Relation != "depends_on" {
		t.Fatalf("unexpected relation value: %s", list[0].Relation)
	}
}
