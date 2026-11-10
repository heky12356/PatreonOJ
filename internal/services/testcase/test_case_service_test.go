package testcase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/models"

	"github.com/minio/minio-go/v7"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeObjectStore struct {
	files map[string]int64
}

func (f *fakeObjectStore) StatObject(_ context.Context, _ string, key string) (minio.ObjectInfo, error) {
	size, ok := f.files[key]
	if !ok {
		return minio.ObjectInfo{}, errors.New("not found")
	}
	return minio.ObjectInfo{
		Key:          key,
		Size:         size,
		LastModified: time.Now(),
	}, nil
}

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

func TestTestCaseService_CommitAndGetByQuestionID(t *testing.T) {
	db := newTestCaseServiceDB(t)
	seedTestCaseQuestion(t, db, 1001, "A")
	store := &fakeObjectStore{files: map[string]int64{
		"problems/1001/1.in":  12,
		"problems/1001/1.out": 8,
	}}
	svc := NewTestCaseService(db, store)

	created, err := svc.CommitOSS(context.Background(), "bucket", TestCaseOSSCommitInput{
		QuestionPK: 1001,
		InputKey:   "problems/1001/1.in",
		OutputKey:  "problems/1001/1.out",
		IsHidden:   false,
	})
	if err != nil {
		t.Fatalf("CommitOSS failed: %v", err)
	}

	list, question, err := svc.GetByQuestionID(1001)
	if err != nil {
		t.Fatalf("GetByQuestionID failed: %v", err)
	}
	if question.Id != 1001 {
		t.Fatalf("unexpected question id: %d", question.Id)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("unexpected list result: %+v", list)
	}
	if list[0].InputKey == "" || list[0].OutputKey == "" {
		t.Fatalf("expected oss keys, got %+v", list[0])
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
	store := &fakeObjectStore{files: map[string]int64{
		"problems/1001/1.in":  12,
		"problems/1001/1.out": 8,
		"problems/1002/2.in":  20,
		"problems/1002/2.out": 16,
	}}
	svc := NewTestCaseService(db, store)

	created, err := svc.CommitOSS(context.Background(), "bucket", TestCaseOSSCommitInput{
		QuestionPK: 1001,
		InputKey:   "problems/1001/1.in",
		OutputKey:  "problems/1001/1.out",
		IsHidden:   false,
	})
	if err != nil {
		t.Fatalf("CommitOSS failed: %v", err)
	}

	updated, err := svc.UpdateOSS(context.Background(), "bucket", created.ID, TestCaseOSSCommitInput{
		QuestionPK: 1002,
		InputKey:   "problems/1002/2.in",
		OutputKey:  "problems/1002/2.out",
		IsHidden:   true,
	})
	if err != nil {
		t.Fatalf("UpdateOSS failed: %v", err)
	}
	if updated.QuestionID != 1002 || updated.IsHidden != true {
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

func TestTestCaseService_CommitOSSInvalidPrefix(t *testing.T) {
	db := newTestCaseServiceDB(t)
	seedTestCaseQuestion(t, db, 1001, "A")
	store := &fakeObjectStore{files: map[string]int64{
		"problems/q-1001/1.in":  12,
		"problems/q-1001/1.out": 8,
	}}
	svc := NewTestCaseService(db, store)

	_, err := svc.CommitOSS(context.Background(), "bucket", TestCaseOSSCommitInput{
		QuestionPK: 1001,
		InputKey:   "problems/q-1001/1.in",
		OutputKey:  "problems/q-1001/1.out",
	})
	if !errors.Is(err, ErrTestCaseInputKeyInvalid) {
		t.Fatalf("expected ErrTestCaseInputKeyInvalid, got %v", err)
	}
}
