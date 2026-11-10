package submission

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSubmissionCommandTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Question{}, &models.Submission{}); err != nil {
		t.Fatalf("migrate test db failed: %v", err)
	}
	return db
}

func seedSubmissionCommandUser(t *testing.T, db *gorm.DB, uuid, status string) models.User {
	t.Helper()
	user := models.User{
		Username: "u_" + uuid,
		Password: "pwd",
		UUID:     uuid,
		Status:   status,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	return user
}

func seedSubmissionCommandQuestion(t *testing.T, db *gorm.DB, number int) models.Question {
	t.Helper()
	q := models.Question{
		Id:         number,
		QuestionId: fmt.Sprintf("q-%d", number),
		Title:      fmt.Sprintf("Question-%d", number),
		Difficulty: "Easy",
		Status:     "published",
	}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("seed question failed: %v", err)
	}
	return q
}

func seedSubmissionRecord(t *testing.T, db *gorm.DB, submission models.Submission) {
	t.Helper()
	if err := db.Create(&submission).Error; err != nil {
		t.Fatalf("seed submission failed: %v", err)
	}
}

func TestSubmissionCommandService_CreateSubmission_UserNotFound(t *testing.T) {
	db := newSubmissionCommandTestDB(t)
	seedSubmissionCommandQuestion(t, db, 1001)
	svc := NewSubmissionCommandService(db, nil)

	_, err := svc.CreateSubmissionByQuestionNumber("missing", 1001, "package main")
	if !errors.Is(err, ErrSubmissionUserNotFound) {
		t.Fatalf("expected ErrSubmissionUserNotFound, got %v", err)
	}
}

func TestSubmissionCommandService_CreateSubmission_UserDisabled(t *testing.T) {
	db := newSubmissionCommandTestDB(t)
	seedSubmissionCommandUser(t, db, "u1", "disabled")
	seedSubmissionCommandQuestion(t, db, 1001)
	svc := NewSubmissionCommandService(db, nil)

	_, err := svc.CreateSubmissionByQuestionNumber("u1", 1001, "package main")
	if !errors.Is(err, ErrSubmissionUserDisabled) {
		t.Fatalf("expected ErrSubmissionUserDisabled, got %v", err)
	}
}

func TestSubmissionCommandService_CreateSubmission_QuestionNotFound(t *testing.T) {
	db := newSubmissionCommandTestDB(t)
	seedSubmissionCommandUser(t, db, "u1", "active")
	svc := NewSubmissionCommandService(db, nil)

	_, err := svc.CreateSubmissionByQuestionNumber("u1", 9999, "package main")
	if !errors.Is(err, ErrSubmissionQuestionNotFound) {
		t.Fatalf("expected ErrSubmissionQuestionNotFound, got %v", err)
	}
}

func TestSubmissionCommandService_CreateSubmission_Success(t *testing.T) {
	db := newSubmissionCommandTestDB(t)
	seedSubmissionCommandUser(t, db, "u1", "active")
	q := seedSubmissionCommandQuestion(t, db, 1001)
	svc := NewSubmissionCommandService(db, nil)

	out, err := svc.CreateSubmissionByQuestionNumber("u1", 1001, "package main\nfunc main(){}")
	if err != nil {
		t.Fatalf("CreateSubmission failed: %v", err)
	}
	if out.Submission == nil {
		t.Fatalf("expected submission not nil")
	}
	if out.QuestionPK != 1001 {
		t.Fatalf("unexpected question pk: %d", out.QuestionPK)
	}
	if out.QuestionID != q.QuestionId {
		t.Fatalf("unexpected question id: %s", out.QuestionID)
	}
	if out.Submission.UserID != "u1" || out.Submission.QuestionID != q.Id {
		t.Fatalf("unexpected submission fields: %+v", out.Submission)
	}
}

func TestSubmissionCommandService_BuildSubmissionResult_NotFound(t *testing.T) {
	db := newSubmissionCommandTestDB(t)
	svc := NewSubmissionCommandService(db, nil)

	_, err := svc.BuildSubmissionResult("missing")
	if !errors.Is(err, ErrSubmissionNotFound) {
		t.Fatalf("expected ErrSubmissionNotFound, got %v", err)
	}
}

func TestSubmissionCommandService_BuildSubmissionResult_Completed(t *testing.T) {
	db := newSubmissionCommandTestDB(t)
	seedSubmissionCommandUser(t, db, "u1", "active")
	q := seedSubmissionCommandQuestion(t, db, 1001)
	resultsJSON, _ := json.Marshal([]models.TestCaseResult{
		{Input: "1", ActualOutput: "1", IsCorrect: true},
		{Input: "2", ActualOutput: "x", IsCorrect: false},
	})
	seedSubmissionRecord(t, db, models.Submission{
		ID:         "s1",
		UserID:     "u1",
		QuestionID: q.Id,
		Status:     "completed",
		Results:    string(resultsJSON),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	svc := NewSubmissionCommandService(db, nil)

	response, err := svc.BuildSubmissionResult("s1")
	if err != nil {
		t.Fatalf("BuildSubmissionResult failed: %v", err)
	}
	if response["question_pk"] != 1001 {
		t.Fatalf("unexpected question pk: %v", response["question_pk"])
	}
	passRate, ok := response["pass_rate"].(float64)
	if !ok || passRate != 0.5 {
		t.Fatalf("unexpected pass rate: %v", response["pass_rate"])
	}
	if response["total_cases"] != 2 || response["passed_cases"] != 1 {
		t.Fatalf("unexpected case stats: total=%v passed=%v", response["total_cases"], response["passed_cases"])
	}
}

func TestSubmissionCommandService_BuildSubmissionResult_Error(t *testing.T) {
	db := newSubmissionCommandTestDB(t)
	seedSubmissionCommandUser(t, db, "u1", "active")
	q := seedSubmissionCommandQuestion(t, db, 1001)
	seedSubmissionRecord(t, db, models.Submission{
		ID:         "s2",
		UserID:     "u1",
		QuestionID: q.Id,
		Status:     "error",
		ErrorCode:  "E002",
		ErrorMsg:   "compile failed",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	svc := NewSubmissionCommandService(db, nil)

	response, err := svc.BuildSubmissionResult("s2")
	if err != nil {
		t.Fatalf("BuildSubmissionResult failed: %v", err)
	}
	if response["error_code"] != "E002" {
		t.Fatalf("unexpected error code: %v", response["error_code"])
	}
	msg, _ := response["message"].(string)
	if msg == "" {
		t.Fatalf("expected non-empty error message")
	}
}

func TestEnsureSubmissionErrorCode(t *testing.T) {
	cases := []struct {
		errMsg string
		want   string
	}{
		{"test case missing", "E001"},
		{"question has no available testcases", "E001"},
		{"compile failed", "E002"},
		{"timeout exceeded", "E003"},
		{"memory exceeded", "E004"},
		{"network unavailable", "E005"},
		{"go-judge api error: 404 - 404 page not found", "E005"},
		{"unknown", "E999"},
	}

	for _, tc := range cases {
		if got := EnsureSubmissionErrorCode(errors.New(tc.errMsg)); got != tc.want {
			t.Fatalf("err=%q got=%s want=%s", tc.errMsg, got, tc.want)
		}
	}
}

func TestParseSubmissionResults(t *testing.T) {
	if got := ParseSubmissionResults(""); len(got) != 0 {
		t.Fatalf("expected empty for empty input")
	}
	if got := ParseSubmissionResults("invalid"); len(got) != 0 {
		t.Fatalf("expected empty for invalid json")
	}
}
