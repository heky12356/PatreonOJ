package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"dachuang/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCategoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Question{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedCategory(t *testing.T, db *gorm.DB, name, slug string, parentID int) models.Category {
	t.Helper()
	c := models.Category{
		Name:     name,
		Slug:     slug,
		ParentID: parentID,
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("seed category failed: %v", err)
	}
	return c
}

func seedQuestionForCategory(t *testing.T, db *gorm.DB, number int) models.Question {
	t.Helper()
	q := models.Question{
		QuestionNumber: number,
		QuestionId:     fmt.Sprintf("q-%d", number),
		Title:          fmt.Sprintf("Question-%d", number),
		Difficulty:     "Easy",
		Status:         "published",
	}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
	return q
}

func TestCategoryService_List(t *testing.T) {
	db := newCategoryTestDB(t)
	svc := NewCategoryService(db)

	category := seedCategory(t, db, "dp", "dynamic-programming", 0)
	question := seedQuestionForCategory(t, db, 1001)
	if err := db.Model(&category).Association("Question").Append(&question); err != nil {
		t.Fatalf("append question to category failed: %v", err)
	}

	list, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 category, got %d", len(list))
	}
	if list[0].QuestionCount != 1 {
		t.Fatalf("expected question_count=1, got %d", list[0].QuestionCount)
	}
}

func TestCategoryService_Create(t *testing.T) {
	db := newCategoryTestDB(t)
	svc := NewCategoryService(db)

	out, err := svc.Create(CategoryCreateInput{
		Name:        "graph",
		Slug:        "graph",
		ParentID:    2,
		Description: "graph related",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if out.Id == 0 || out.Name != "graph" || out.ParentID != 2 {
		t.Fatalf("unexpected category: %+v", out)
	}
}

func TestCategoryService_UpdateNotFound(t *testing.T) {
	db := newCategoryTestDB(t)
	svc := NewCategoryService(db)

	_, err := svc.Update(9999, CategoryUpdateInput{})
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestCategoryService_Update(t *testing.T) {
	db := newCategoryTestDB(t)
	svc := NewCategoryService(db)

	category := seedCategory(t, db, "old", "old", 0)
	name := "new"
	description := "updated"
	parentID := 10

	out, err := svc.Update(category.Id, CategoryUpdateInput{
		Name:        &name,
		Description: &description,
		ParentID:    &parentID,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if out.Name != "new" || out.Description != "updated" || out.ParentID != 10 {
		t.Fatalf("unexpected updated category: %+v", out)
	}
}

func TestCategoryService_Delete(t *testing.T) {
	db := newCategoryTestDB(t)
	svc := NewCategoryService(db)

	category := seedCategory(t, db, "to-del", "to-del", 0)
	if err := svc.Delete(category.Id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	err := svc.Delete(category.Id)
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("expected ErrCategoryNotFound after delete, got %v", err)
	}
}
