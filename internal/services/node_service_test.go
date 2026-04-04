package services

import (
	"fmt"
	"strings"
	"testing"

	"dachuang/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newNodeServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func TestNodeService_List(t *testing.T) {
	db := newNodeServiceTestDB(t)
	if err := db.Create(&models.Node{
		Name:    "Two Sum",
		Type:    "question",
		Content: "example",
	}).Error; err != nil {
		t.Fatalf("seed node failed: %v", err)
	}

	svc := NewNodeService(db)
	list, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 node, got %d", len(list))
	}
	if list[0].Name != "Two Sum" {
		t.Fatalf("unexpected node name: %s", list[0].Name)
	}
}
