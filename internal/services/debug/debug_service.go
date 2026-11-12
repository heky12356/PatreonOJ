package debug

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"patreonoj/internal/models"
	"patreonoj/internal/services/judge"
	"patreonoj/internal/util"
)

var (
	ErrInvalid          = errors.New("invalid self-test request")
	ErrNotFound         = errors.New("self-test not found")
	ErrQuestionNotFound = errors.New("question not found")
	ErrBusy             = errors.New("self-test resources busy")
	ErrUnavailable      = errors.New("self-test service unavailable")
)

const MaxCode = 256 << 10
const MaxInput = 64 << 10
const retention = 15 * time.Minute

type Runner interface {
	DebugContext(context.Context, string, string, string, *string, int64, int64) (*judge.DebugResult, error)
}
type ObjectStore interface {
	GetObjectBytes(context.Context, string, string) ([]byte, error)
}

type Request struct {
	QuestionPK int     `json:"question_pk"`
	Language   string  `json:"language"`
	Code       string  `json:"code"`
	Input      string  `json:"input"`
	Expected   *string `json:"expected_output"`
}

type Result struct {
	ID         string `json:"id"`
	QuestionPK int    `json:"question_pk"`
	Language   string `json:"language"`
	Status     string `json:"status"`
	judge.DebugResult
	CreatedAt time.Time `json:"created_at"`
}

type record struct {
	result           Result
	owner            string
	request          Request
	timeMS, memoryMB int64
	cancel           context.CancelFunc
	finished         time.Time
}

type Service struct {
	db      *gorm.DB
	runner  Runner
	objects ObjectStore
	bucket  string
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	queue   chan string
	mu      sync.Mutex
	runs    map[string]*record
	active  map[string]string
}

func NewService(db *gorm.DB, runner Runner, objects ObjectStore, bucket string) *Service {
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{db: db, runner: runner, objects: objects, bucket: bucket, ctx: ctx, cancel: cancel, done: make(chan struct{}), queue: make(chan string, 4), runs: map[string]*record{}, active: map[string]string{}}
	go s.worker()
	return s
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) question(ctx context.Context, user models.User, pk int) (models.Question, error) {
	var q models.Question
	err := s.db.WithContext(ctx).Where("id = ?", pk).First(&q).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return q, ErrQuestionNotFound
	}
	if err != nil {
		return q, err
	}
	if q.Status != "published" && !util.UserInstance.HasPermission(user, "admin") {
		return q, ErrQuestionNotFound
	}
	return q, nil
}

func (s *Service) Create(ctx context.Context, user models.User, request Request) (Result, error) {
	if s.runner == nil || s.ctx.Err() != nil {
		return Result{}, ErrUnavailable
	}
	request.Language = strings.ToLower(strings.TrimSpace(request.Language))
	if request.QuestionPK < 1 || user.UUID == "" || strings.TrimSpace(request.Code) == "" || len(request.Code) > MaxCode || len(request.Input) > MaxInput || (request.Expected != nil && len(*request.Expected) > MaxInput) {
		return Result{}, ErrInvalid
	}
	switch request.Language {
	case "python", "cpp", "java", "go":
	default:
		return Result{}, ErrInvalid
	}
	q, err := s.question(ctx, user, request.QuestionPK)
	if err != nil {
		return Result{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if _, ok := s.active[user.UUID]; ok {
		return Result{}, ErrBusy
	}
	if len(s.runs) >= 64 {
		oldestID := ""
		var oldest time.Time
		for id, run := range s.runs {
			if !run.finished.IsZero() && (oldestID == "" || run.finished.Before(oldest)) {
				oldestID = id
				oldest = run.finished
			}
		}
		if oldestID == "" {
			return Result{}, ErrBusy
		}
		delete(s.runs, oldestID)
	}
	id := uuid.NewString()
	r := &record{result: Result{ID: id, QuestionPK: q.Id, Language: request.Language, Status: "pending", CreatedAt: time.Now().UTC()}, owner: user.UUID, request: request, timeMS: int64(min(5000, max(100, q.TimeLimit))), memoryMB: int64(min(512, max(64, q.MemoryLimit)))}
	s.runs[id] = r
	s.active[user.UUID] = id
	select {
	case s.queue <- id:
		return r.result, nil
	default:
		delete(s.runs, id)
		delete(s.active, user.UUID)
		return Result{}, ErrBusy
	}
}

func (s *Service) prune() {
	for id, r := range s.runs {
		if !r.finished.IsZero() && time.Since(r.finished) > retention {
			delete(s.runs, id)
		}
	}
}

func (s *Service) Get(owner, id string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	r, ok := s.runs[id]
	if !ok || r.owner != owner {
		return Result{}, ErrNotFound
	}
	return r.result, nil
}

func (s *Service) Cancel(owner, id string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok || r.owner != owner {
		return Result{}, ErrNotFound
	}
	if r.result.Status == "pending" || r.result.Status == "running" {
		if r.cancel != nil {
			r.cancel()
		}
		r.result.Status = "cancelled"
		r.result.Verdict = "CANCELLED"
		r.finished = time.Now()
		r.request = Request{}
		if s.active[owner] == id {
			delete(s.active, owner)
		}
	}
	return r.result, nil
}

func (s *Service) worker() {
	defer close(s.done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case id := <-s.queue:
			s.mu.Lock()
			r := s.runs[id]
			if r == nil || r.result.Status != "pending" {
				s.mu.Unlock()
				continue
			}
			ctx, cancel := context.WithTimeout(s.ctx, 120*time.Second)
			r.cancel = cancel
			r.result.Status = "running"
			request := r.request
			timeMS, memoryMB := r.timeMS, r.memoryMB
			s.mu.Unlock()
			result, err := s.runner.DebugContext(ctx, request.Code, request.Language, request.Input, request.Expected, timeMS, memoryMB)
			cancel()
			s.mu.Lock()
			if r.result.Status != "cancelled" {
				r.result.Status = "completed"
				if err != nil {
					var compile *judge.CompileError
					r.result.Verdict = "SE"
					r.result.Stderr = "自测服务暂不可用，请稍后重试。"
					if errors.As(err, &compile) {
						r.result.Verdict = "CE"
						r.result.Stderr = compile.Message
					}
				} else if result != nil {
					r.result.DebugResult = *result
				}
			}
			r.request = Request{}
			r.cancel = nil
			r.finished = time.Now()
			if s.active[r.owner] == id {
				delete(s.active, r.owner)
			}
			s.mu.Unlock()
		}
	}
}

type Sample struct {
	ID     uint   `json:"id"`
	Input  string `json:"input"`
	Output string `json:"output"`
}

type Samples struct {
	Samples   []Sample `json:"samples"`
	Truncated bool     `json:"truncated"`
}

func (s *Service) Samples(ctx context.Context, user models.User, pk int) (Samples, error) {
	result := Samples{Samples: []Sample{}}
	if _, err := s.question(ctx, user, pk); err != nil {
		return result, err
	}
	if s.objects == nil {
		return result, ErrUnavailable
	}
	var cases []models.TestCase
	if err := s.db.WithContext(ctx).Where("question_id = ? AND is_hidden = ?", pk, false).Order("id ASC").Limit(6).Find(&cases).Error; err != nil {
		return result, err
	}
	if len(cases) > 5 {
		result.Truncated = true
		cases = cases[:5]
	}
	for _, c := range cases {
		if c.InputSize > MaxInput || c.OutputSize > MaxInput {
			result.Truncated = true
			continue
		}
		input, err := s.objects.GetObjectBytes(ctx, s.bucket, c.InputKey)
		if err != nil {
			return result, ErrUnavailable
		}
		output, err := s.objects.GetObjectBytes(ctx, s.bucket, c.OutputKey)
		if err != nil {
			return result, ErrUnavailable
		}
		if len(input) > MaxInput || len(output) > MaxInput {
			result.Truncated = true
			continue
		}
		result.Samples = append(result.Samples, Sample{ID: c.ID, Input: string(input), Output: string(output)})
	}
	return result, nil
}
