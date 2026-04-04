package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"dachuang/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSubmissionQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Question{}, &models.Submission{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedSubmissionQuestion(t *testing.T, db *gorm.DB, number int, title string) models.Question {
	t.Helper()
	q := models.Question{
		QuestionNumber: number,
		QuestionId:     fmt.Sprintf("q-%d", number),
		Title:          title,
		Difficulty:     "Easy",
		Status:         "published",
	}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
	return q
}

func seedSubmission(t *testing.T, db *gorm.DB, id, userID string, questionID int, status, language string, createdAt time.Time) {
	t.Helper()
	s := models.Submission{
		ID:         id,
		UserID:     userID,
		QuestionID: questionID,
		Status:     status,
		Language:   language,
		CodeLength: 10,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("seed submission failed: %v", err)
	}
}

func TestSubmissionQueryService_ListProblemSubmissions_NotFound(t *testing.T) {
	db := newSubmissionQueryTestDB(t)
	svc := NewSubmissionQueryService(db)

	_, err := svc.ListProblemSubmissions(9999, "", "", 1, 20)
	if !errors.Is(err, ErrSubmissionQuestionNotFound) {
		t.Fatalf("expected ErrSubmissionQuestionNotFound, got %v", err)
	}
}

func TestSubmissionQueryService_ListProblemSubmissions_Success(t *testing.T) {
	db := newSubmissionQueryTestDB(t)
	q := seedSubmissionQuestion(t, db, 1001, "A")
	now := time.Now()
	seedSubmission(t, db, "s1", "u1", q.Id, "completed", "go", now.Add(-time.Minute))
	seedSubmission(t, db, "s2", "u2", q.Id, "error", "go", now)

	svc := NewSubmissionQueryService(db)
	page, err := svc.ListProblemSubmissions(1001, "completed", "go", 1, 20)
	if err != nil {
		t.Fatalf("ListProblemSubmissions failed: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("unexpected page result: total=%d items=%d", page.Total, len(page.Items))
	}
	if page.Items[0].SubmissionID != "s1" {
		t.Fatalf("unexpected submission id: %s", page.Items[0].SubmissionID)
	}
}

func TestSubmissionQueryService_ListUserSubmissions_InvalidProblemID(t *testing.T) {
	db := newSubmissionQueryTestDB(t)
	svc := NewSubmissionQueryService(db)

	_, err := svc.ListUserSubmissions("u1", "abc", "", "", 1, 20)
	if !errors.Is(err, ErrSubmissionProblemIDInvalid) {
		t.Fatalf("expected ErrSubmissionProblemIDInvalid, got %v", err)
	}
}

func TestSubmissionQueryService_ListUserSubmissions_ResolveProblemNumber(t *testing.T) {
	db := newSubmissionQueryTestDB(t)
	q := seedSubmissionQuestion(t, db, 2001, "B")
	now := time.Now()
	seedSubmission(t, db, "s3", "u1", q.Id, "completed", "python", now)
	seedSubmission(t, db, "s4", "u2", q.Id, "completed", "python", now)

	svc := NewSubmissionQueryService(db)
	page, err := svc.ListUserSubmissions("u1", "2001", "", "", 1, 20)
	if err != nil {
		t.Fatalf("ListUserSubmissions failed: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("unexpected result: total=%d items=%d", page.Total, len(page.Items))
	}
	if page.Items[0].SubmissionID != "s3" {
		t.Fatalf("unexpected submission id: %s", page.Items[0].SubmissionID)
	}
}
