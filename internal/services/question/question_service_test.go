package question

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
	if err := db.AutoMigrate(&models.Question{}, &models.TestCase{}, &models.QuestionTag{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedQuestionSvc(t *testing.T, db *gorm.DB, number int, title, status string) models.Question {
	t.Helper()
	q := models.Question{
		Id:         number,
		QuestionId: fmt.Sprintf("q-%d", number),
		Title:      title,
		Difficulty: "Easy",
		Status:     status,
	}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
	return q
}

func TestQuestionService_List_SearchByTitleTagsContentAndID(t *testing.T) {
	db := newQuestionServiceTestDB(t)
	q1 := seedQuestionSvc(t, db, 1001, "Binary Search", "published")
	q2 := seedQuestionSvc(t, db, 1002, "Dynamic Programming", "published")

	if err := db.Model(&q1).Updates(map[string]any{
		"tags":    "array,binary-search",
		"content": "find target in sorted array",
	}).Error; err != nil {
		t.Fatalf("update q1 failed: %v", err)
	}
	if err := db.Model(&q2).Updates(map[string]any{
		"tags":    "dp,knapsack",
		"content": "optimize transitions",
	}).Error; err != nil {
		t.Fatalf("update q2 failed: %v", err)
	}

	svc := NewQuestionService(db)

	byTag, err := svc.List("binary-search", "", false, 1, 10)
	if err != nil {
		t.Fatalf("List by tag failed: %v", err)
	}
	if len(byTag.Result) != 1 || byTag.Result[0].Id != q1.Id {
		t.Fatalf("unexpected tag search result: %+v", byTag.Result)
	}

	byContent, err := svc.List("transitions", "", false, 1, 10)
	if err != nil {
		t.Fatalf("List by content failed: %v", err)
	}
	if len(byContent.Result) != 1 || byContent.Result[0].Id != q2.Id {
		t.Fatalf("unexpected content search result: %+v", byContent.Result)
	}

	byID, err := svc.List("1001", "", false, 1, 10)
	if err != nil {
		t.Fatalf("List by numeric id failed: %v", err)
	}
	if len(byID.Result) != 1 || byID.Result[0].Id != q1.Id {
		t.Fatalf("unexpected id search result: %+v", byID.Result)
	}
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

func TestQuestionService_Create_AutoNumberAndDuplicateQuestionIDAllowed(t *testing.T) {
	db := newQuestionServiceTestDB(t)
	seedQuestionSvc(t, db, 1001, "base", "published")
	svc := NewQuestionService(db)

	created, err := svc.Create(QuestionCreateInput{
		QuestionID: "q-1002",
		Title:      "new question",
		Difficulty: "Medium",
	})
	if err != nil {
		t.Fatalf("Create auto number failed: %v", err)
	}
	if created.Id == 0 {
		t.Fatalf("expected auto generated id")
	}

	_, err = svc.Create(QuestionCreateInput{
		QuestionID: "q-1001",
		Title:      "dup",
		Difficulty: "Easy",
	})
	if err != nil {
		t.Fatalf("expected duplicate question_id to be allowed, got %v", err)
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
	if err := db.Create(&models.TestCase{QuestionID: question.Id, InputKey: "in", OutputKey: "out"}).Error; err != nil {
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
