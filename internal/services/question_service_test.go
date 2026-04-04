package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newQuestionServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Question{}, &models.TestCase{}, &models.QuestionCategory{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedQuestionSvc(t *testing.T, db *gorm.DB, number int, title, status string) models.Question {
	t.Helper()
	q := models.Question{
		QuestionNumber: number,
		QuestionId:     fmt.Sprintf("q-%d", number),
		Title:          title,
		Difficulty:     "Easy",
		Status:         status,
	}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
	return q
}

func TestQuestionService_List_AdminAndNonAdmin(t *testing.T) {
	db := newQuestionServiceTestDB(t)
	seedQuestionSvc(t, db, 1001, "A", "draft")
	seedQuestionSvc(t, db, 1002, "B", "published")
	svc := NewQuestionService(db)

	adminResult, err := svc.List("%", "", true, 1, 10)
	if err != nil {
		t.Fatalf("List admin failed: %v", err)
	}
	if adminResult.TotalCnt != 2 {
		t.Fatalf("expected 2 for admin, got %d", adminResult.TotalCnt)
	}

	userResult, err := svc.List("%", "", false, 1, 10)
	if err != nil {
		t.Fatalf("List non-admin failed: %v", err)
	}
	if userResult.TotalCnt != 1 {
		t.Fatalf("expected 1 for non-admin, got %d", userResult.TotalCnt)
	}
}

func TestQuestionService_Create_AutoNumberAndDuplicate(t *testing.T) {
	db := newQuestionServiceTestDB(t)
	seedQuestionSvc(t, db, 1001, "base", "published")
	svc := NewQuestionService(db)

	created, err := svc.Create(QuestionCreateInput{
		Title:      "new question",
		Difficulty: "Medium",
	})
	if err != nil {
		t.Fatalf("Create auto number failed: %v", err)
	}
	if created.QuestionNumber != 1002 {
		t.Fatalf("expected auto number 1002, got %d", created.QuestionNumber)
	}

	_, err = svc.Create(QuestionCreateInput{
		QuestionNumber: 1001,
		Title:          "dup",
		Difficulty:     "Easy",
	})
	if !errors.Is(err, ErrQuestionNumberExists) {
		t.Fatalf("expected ErrQuestionNumberExists, got %v", err)
	}
}

func TestQuestionService_GetAndUpdateByNumber(t *testing.T) {
	db := newQuestionServiceTestDB(t)
	seedQuestionSvc(t, db, 1001, "before", "published")
	svc := NewQuestionService(db)

	got, err := svc.GetByNumber(1001)
	if err != nil {
		t.Fatalf("GetByNumber failed: %v", err)
	}
	if got.Title != "before" {
		t.Fatalf("unexpected title: %s", got.Title)
	}

	updated, err := svc.UpdateByNumber(1001, models.Question{
		Title: "after",
	})
	if err != nil {
		t.Fatalf("UpdateByNumber failed: %v", err)
	}
	if updated.Title != "after" {
		t.Fatalf("expected updated title, got %s", updated.Title)
	}

	_, err = svc.GetByNumber(9999)
	if !errors.Is(err, ErrQuestionRecordNotFound) {
		t.Fatalf("expected ErrQuestionRecordNotFound, got %v", err)
	}
}

func TestQuestionService_DeleteByNumber(t *testing.T) {
	db := newQuestionServiceTestDB(t)
	question := seedQuestionSvc(t, db, 1001, "to-del", "published")
	if err := db.Create(&models.TestCase{QuestionID: question.Id, Input: "1", ExpectedOutput: "1"}).Error; err != nil {
		t.Fatalf("seed testcase failed: %v", err)
	}
	svc := NewQuestionService(db)

	if err := svc.DeleteByNumber(1001); err != nil {
		t.Fatalf("DeleteByNumber failed: %v", err)
	}

	var testCaseCount int64
	if err := db.Model(&models.TestCase{}).Where("question_id = ?", question.Id).Count(&testCaseCount).Error; err != nil {
		t.Fatalf("count testcases failed: %v", err)
	}
	if testCaseCount != 0 {
		t.Fatalf("expected testcase deleted, count=%d", testCaseCount)
	}

	if err := svc.DeleteByNumber(1001); !errors.Is(err, ErrQuestionRecordNotFound) {
		t.Fatalf("expected ErrQuestionRecordNotFound, got %v", err)
	}
}
