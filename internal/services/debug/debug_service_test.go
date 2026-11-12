package debug

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"patreonoj/internal/models"
	"patreonoj/internal/services/judge"
	"sync"
	"testing"
	"time"
)

type runnerFunc func(context.Context, string, string, string, *string, int64, int64) (*judge.DebugResult, error)

func (f runnerFunc) DebugContext(c context.Context, code, lang, input string, expected *string, t, m int64) (*judge.DebugResult, error) {
	return f(c, code, lang, input, expected, t, m)
}

type objects struct {
	mu   sync.Mutex
	keys []string
}

func (o *objects) GetObjectBytes(_ context.Context, _, key string) ([]byte, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.keys = append(o.keys, key)
	return []byte(key), nil
}

func setup(t *testing.T, runner Runner) (*Service, *gorm.DB, *objects) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Question{}, &models.TestCase{}, &models.Submission{}, &models.UserSolvedQuestion{}, &models.UserSkillMastery{}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []models.Question{{Id: 1, QuestionId: "p1", Title: "Public", Status: "published", TimeLimit: 999999, MemoryLimit: 9999}, {Id: 2, QuestionId: "p2", Title: "Draft", Status: "draft"}} {
		if err := db.Create(&q).Error; err != nil {
			t.Fatal(err)
		}
	}
	store := &objects{}
	s := NewService(db, runner, store, "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Shutdown(ctx)
		sql, _ := db.DB()
		sql.Close()
	})
	return s, db, store
}

func await(t *testing.T, s *Service, owner, id string) Result {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r, err := s.Get(owner, id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status == "completed" || r.Status == "cancelled" {
			return r
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("self-test did not finish")
	return Result{}
}

func TestRunUsesOnlyCustomInputAndPreservesProgress(t *testing.T) {
	runner := runnerFunc(func(_ context.Context, code, lang, input string, expected *string, tm, mm int64) (*judge.DebugResult, error) {
		if code != "print(3)" || lang != "python" || input != "" || expected == nil || *expected != "3\n" || tm != 5000 || mm != 512 {
			return nil, errors.New("unexpected sandbox request")
		}
		return &judge.DebugResult{Verdict: "OK", Stdout: "3\n", Stderr: "diagnostic", Comparison: "match"}, nil
	})
	s, db, store := setup(t, runner)
	expected := "3\n"
	r, err := s.Create(context.Background(), models.User{Id: 1, UUID: "alice"}, Request{QuestionPK: 1, Code: "print(3)", Language: "python", Expected: &expected})
	if err != nil {
		t.Fatal(err)
	}
	result := await(t, s, "alice", r.ID)
	if result.Verdict != "OK" || result.Stdout != "3\n" || result.Stderr != "diagnostic" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(store.keys) != 0 {
		t.Fatal("self-test accessed stored test data")
	}
	for _, model := range []interface{}{&models.Submission{}, &models.UserSolvedQuestion{}, &models.UserSkillMastery{}} {
		var n int64
		db.Model(model).Count(&n)
		if n != 0 {
			t.Fatal("self-test changed submission or learning records")
		}
	}
	if _, err := s.Get("bob", r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("another user read the result")
	}
	if _, err := s.Cancel("bob", r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("another user cancelled the run")
	}
}

func TestCancelAndUserConcurrency(t *testing.T) {
	started := make(chan struct{}, 1)
	runner := runnerFunc(func(ctx context.Context, _, _, _ string, _ *string, _, _ int64) (*judge.DebugResult, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	s, _, _ := setup(t, runner)
	user := models.User{Id: 1, UUID: "alice"}
	request := Request{QuestionPK: 1, Language: "python", Code: "while True: pass"}
	r, err := s.Create(context.Background(), user, request)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	if _, err := s.Create(context.Background(), user, request); !errors.Is(err, ErrBusy) {
		t.Fatal("concurrent user runs allowed")
	}
	if _, err := s.Cancel(user.UUID, r.ID); err != nil {
		t.Fatal(err)
	}
	if result := await(t, s, user.UUID, r.ID); result.Verdict != "CANCELLED" {
		t.Fatal("cancel was lost")
	}
	if _, err := s.Create(context.Background(), user, request); err != nil {
		t.Fatal("cancel did not release user slot")
	}
}

func TestSamplesHidePrivateDataAndRespectDraftVisibility(t *testing.T) {
	s, db, store := setup(t, nil)
	for _, tc := range []models.TestCase{{QuestionID: 1, InputKey: "sample.in", OutputKey: "sample.out"}, {QuestionID: 1, InputKey: "secret.in", OutputKey: "secret.out", IsHidden: true}} {
		if err := db.Create(&tc).Error; err != nil {
			t.Fatal(err)
		}
	}
	result, err := s.Samples(context.Background(), models.User{}, 1)
	if err != nil || len(result.Samples) != 1 || result.Samples[0].Input != "sample.in" {
		t.Fatalf("samples: %+v %v", result, err)
	}
	for _, key := range store.keys {
		if key == "secret.in" || key == "secret.out" {
			t.Fatal("hidden testcase was read")
		}
	}
	if _, err := s.Samples(context.Background(), models.User{}, 2); !errors.Is(err, ErrQuestionNotFound) {
		t.Fatal("draft samples exposed")
	}
}

func TestCompileErrorAndValidation(t *testing.T) {
	runner := runnerFunc(func(context.Context, string, string, string, *string, int64, int64) (*judge.DebugResult, error) {
		return nil, &judge.CompileError{Message: "main.cpp: syntax error"}
	})
	s, _, _ := setup(t, runner)
	user := models.User{Id: 1, UUID: "alice"}
	for _, req := range []Request{{QuestionPK: 1, Language: "bash", Code: "echo hi"}, {QuestionPK: 1, Language: "python", Code: ""}, {QuestionPK: 1, Language: "python", Code: "pass", Input: string(make([]byte, MaxInput+1))}} {
		if _, err := s.Create(context.Background(), user, req); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid request accepted")
		}
	}
	if _, err := s.Create(context.Background(), user, Request{QuestionPK: 2, Language: "python", Code: "pass"}); !errors.Is(err, ErrQuestionNotFound) {
		t.Fatal("ordinary user ran a draft question")
	}
	r, err := s.Create(context.Background(), user, Request{QuestionPK: 1, Language: "cpp", Code: "broken"})
	if err != nil {
		t.Fatal(err)
	}
	result := await(t, s, user.UUID, r.ID)
	if result.Verdict != "CE" || result.Stderr != "main.cpp: syntax error" {
		t.Fatalf("compile diagnostic lost: %+v", result)
	}
}
