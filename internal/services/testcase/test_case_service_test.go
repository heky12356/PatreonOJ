package testcase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestCaseServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Question{}, &models.TestCase{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedTestCaseQuestion(t *testing.T, db *gorm.DB, number int, title string) models.Question {
	t.Helper()
	q := models.Question{
		Id:         number,
		QuestionId: fmt.Sprintf("q-%d", number),
		Title:      title,
		Difficulty: "Easy",
		Status:     "published",
	}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
	return q
}

func TestTestCaseService_CreateAndGetByQuestionID(t *testing.T) {
	db := newTestCaseServiceDB(t)
	seedTestCaseQuestion(t, db, 1001, "A")
	svc := NewTestCaseService(db, nil)

	created, err := svc.Create(TestCaseCreateInput{
		QuestionPK:     1001,
		Input:          "1 2",
		ExpectedOutput: "3",
		IsHidden:       false,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	list, question, err := svc.GetByQuestionID(1001)
	if err != nil {
		t.Fatalf("GetByQuestionID failed: %v", err)
	}
	if question.Id != 1001 {
		t.Fatalf("unexpected question number: %d", question.Id)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("unexpected list result: %+v", list)
	}
}

func TestTestCaseService_ListByQuestionNotFound(t *testing.T) {
	db := newTestCaseServiceDB(t)
	svc := NewTestCaseService(db, nil)

	qn := 9999
	_, err := svc.List(&qn)
	if !errors.Is(err, ErrTestCaseQuestionNotFound) {
		t.Fatalf("expected ErrTestCaseQuestionNotFound, got %v", err)
	}
}

func TestTestCaseService_UpdateDeleteFlow(t *testing.T) {
	db := newTestCaseServiceDB(t)
	seedTestCaseQuestion(t, db, 1001, "A")
	seedTestCaseQuestion(t, db, 1002, "B")
	svc := NewTestCaseService(db, nil)

	created, err := svc.Create(TestCaseCreateInput{
		QuestionPK:     1001,
		Input:          "1",
		ExpectedOutput: "1",
		IsHidden:       false,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := svc.Update(created.ID, TestCaseCreateInput{
		QuestionPK:     1002,
		Input:          "2",
		ExpectedOutput: "2",
		IsHidden:       true,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Input != "2" || updated.IsHidden != true {
		t.Fatalf("unexpected updated testcase: %+v", updated)
	}

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = svc.GetByID(created.ID)
	if !errors.Is(err, ErrTestCaseNotFound) {
		t.Fatalf("expected ErrTestCaseNotFound, got %v", err)
	}
}

func TestTestCaseService_CommitOSSWithoutClient(t *testing.T) {
	db := newTestCaseServiceDB(t)
	seedTestCaseQuestion(t, db, 1001, "A")
	svc := NewTestCaseService(db, nil)

	_, err := svc.CommitOSS(context.Background(), "bucket", TestCaseOSSCommitInput{
		QuestionPK: 1001,
		InputKey:   "1.in",
		OutputKey:  "1.out",
	})
	if !errors.Is(err, ErrTestCaseOSSUnavailable) {
		t.Fatalf("expected ErrTestCaseOSSUnavailable, got %v", err)
	}
}
