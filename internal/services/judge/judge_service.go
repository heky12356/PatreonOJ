package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	assessmentsvc "patreonoj/internal/services/assessment"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSubmissionStale = errors.New("submission is no longer owned by this judge attempt")

type JudgeService struct {
	DB                *gorm.DB
	HTTPClient        *http.Client
	LocalJudgeService *LocalJudgeService
	GoJudgeClient     *GoJudgeClient
	Config            *config.JudgeConfig
	OSSClient         *oss.OSS
	OSSBucket         string
	AssessmentService *assessmentsvc.AssessmentService
}

func NewJudgeService(cfg *config.JudgeConfig, db *gorm.DB, ossClient *oss.OSS, ossBucket string, assessmentService *assessmentsvc.AssessmentService) *JudgeService {
	if cfg == nil {
		cfg = &config.JudgeConfig{Timeout: 15}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15
	}
	service := &JudgeService{DB: db, Config: cfg, OSSClient: ossClient, OSSBucket: ossBucket, AssessmentService: assessmentService, HTTPClient: &http.Client{Timeout: time.Duration(timeout) * time.Second}}
	if cfg.Local.Enabled {
		service.LocalJudgeService = NewLocalJudgeService(&cfg.Local)
	}
	if cfg.GoJudge.Enabled {
		service.GoJudgeClient = NewGoJudgeClient(cfg.GoJudge.APIURL, timeout)
		service.GoJudgeClient.Token = cfg.GoJudge.Token
	}
	return service
}

func (js *JudgeService) JudgeCode(submission *models.Submission) error {
	return js.JudgeCodeContext(context.Background(), submission)
}

func (js *JudgeService) JudgeCodeContext(ctx context.Context, submission *models.Submission) error {
	if js == nil || js.DB == nil || js.Config == nil || submission == nil {
		return errors.New("judge service is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var current models.Submission
	if err := js.DB.WithContext(ctx).Where("id = ?", submission.ID).First(&current).Error; err != nil {
		return err
	}
	if current.Status == "completed" {
		*submission = current
		return nil
	}
	if current.Status == "pending" {
		claimed := js.DB.WithContext(ctx).Model(&models.Submission{}).Where("id = ? AND status = ? AND updated_at = ?", current.ID, "pending", current.UpdatedAt).Update("status", "processing")
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected != 1 {
			return ErrSubmissionStale
		}
		if err := js.DB.WithContext(ctx).Where("id = ?", current.ID).First(&current).Error; err != nil {
			return err
		}
		*submission = current
	} else if current.Status != "processing" || submission.Status != "processing" || !current.UpdatedAt.Equal(submission.UpdatedAt) {
		return ErrSubmissionStale
	}
	lease := current.UpdatedAt
	var question models.Question
	if err := js.DB.WithContext(ctx).Where("id = ?", current.QuestionID).First(&question).Error; err != nil {
		return fmt.Errorf("load submission question: %w", err)
	}
	testCases, err := js.getTestCasesContext(ctx, current.QuestionID)
	if err != nil {
		return err
	}
	language := strings.ToLower(strings.TrimSpace(current.Language))
	if language == "" {
		language = js.detectLanguage(current.Code)
	}
	if language != "go" && language != "cpp" && language != "python" && language != "java" {
		return fmt.Errorf("unsupported language: %s", language)
	}
	inputs, expected := make([]string, 0, len(testCases)), make([]string, 0, len(testCases))
	totalBytes := 0
	for _, tc := range testCases {
		in, out, err := js.loadTestCaseIO(ctx, tc)
		if err != nil {
			return err
		}
		totalBytes += len(in) + len(out)
		if totalBytes > 64*1024*1024 {
			return errors.New("testcase input/output exceeds submission size limit")
		}
		inputs = append(inputs, in)
		expected = append(expected, normalizeOutput(out))
	}
	timeLimit, memoryLimit := js.problemLimits(question)
	results, err := js.executeJudgement(ctx, current.Code, language, inputs, timeLimit, memoryLimit)
	if err != nil {
		return fmt.Errorf("execute judgement failed: %w", err)
	}
	if len(results) != len(testCases) {
		return fmt.Errorf("judge response count mismatch: got=%d want=%d", len(results), len(testCases))
	}
	allCorrect := len(results) > 0
	verdict := "AC"
	var runtime, memory int64
	for i := range results {
		r := &results[i]
		r.IsCorrect = r.IsCorrect && normalizeOutput(r.ActualOutput) == expected[i]
		r.Verdict = r.ResultVerdict()
		if verdict == "AC" && !r.IsCorrect {
			verdict = r.Verdict
		}
		r.Input = inputs[i]
		r.ExpectedOutput = expected[i]
		r.TestCaseID = testCases[i].ID
		r.Hidden = testCases[i].IsHidden
		if r.Hidden {
			r.Input = ""
			r.ExpectedOutput = ""
			r.ActualOutput = ""
		}
		allCorrect = allCorrect && r.IsCorrect
		runtime = max(runtime, r.Runtime)
		memory = max(memory, r.MemoryUsage)
	}
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	err = js.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated := tx.Model(&models.Submission{}).Where("id = ? AND status = ? AND updated_at = ?", current.ID, "processing", lease).Updates(map[string]interface{}{
			"status": "completed", "verdict": verdict, "results": string(data), "runtime_ms": runtime, "memory_kb": memory, "language": language, "code_length": len(current.Code), "error_code": "", "error_msg": "",
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrSubmissionStale
		}
		if !allCorrect {
			return nil
		}
		solved := models.UserSolvedQuestion{UserUUID: current.UserID, QuestionID: current.QuestionID, SolvedAt: time.Now()}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_uuid"}, {Name: "question_id"}}, DoNothing: true}).Create(&solved).Error; err != nil {
			return err
		}
		return assessmentsvc.NewAssessmentService(tx).UpdateUserMasteryBasedOnResult(current.UserID, current.QuestionID, true)
	})
	if err != nil {
		return fmt.Errorf("save judgement transaction: %w", err)
	}
	return js.DB.WithContext(ctx).Where("id = ?", current.ID).First(submission).Error
}

func (js *JudgeService) executeJudgement(ctx context.Context, code, language string, inputs []string, timeLimit, memoryLimit int) ([]models.TestCaseResult, error) {
	local := func() ([]models.TestCaseResult, error) {
		if js.LocalJudgeService == nil || js.LocalJudgeService.Config == nil {
			return nil, errors.New("local judge is not configured")
		}
		cfg := *js.LocalJudgeService.Config
		cfg.MaxTime = timeLimit
		cfg.MaxMemory = memoryLimit
		return NewLocalJudgeService(&cfg).JudgeBatchContext(ctx, code, inputs, language)
	}
	if js.Config.Mode == "local" {
		return local()
	}
	if js.GoJudgeClient == nil {
		return nil, errors.New("go-judge client is not initialized")
	}
	results, err := js.GoJudgeClient.RunContext(ctx, code, language, inputs, int64(timeLimit), int64(memoryLimit))
	if err == nil {
		return results, nil
	}
	var compileErr *CompileError
	if errors.As(err, &compileErr) || ctx.Err() != nil || js.LocalJudgeService == nil || strings.ToLower(js.LocalJudgeService.Config.Executor) != "docker" {
		return nil, err
	}
	log.Printf("remote judge unavailable, using configured docker judge: %v", err)
	return local()
}

func (js *JudgeService) problemLimits(question models.Question) (int, int) {
	timeLimit, memoryLimit := question.TimeLimit, question.MemoryLimit
	if timeLimit <= 0 {
		timeLimit = js.Config.GoJudge.MaxTime
		if js.Config.Mode == "local" {
			timeLimit = js.Config.Local.MaxTime
		}
	}
	if memoryLimit <= 0 {
		memoryLimit = js.Config.GoJudge.MaxMemory
		if js.Config.Mode == "local" {
			memoryLimit = js.Config.Local.MaxMemory
		}
	}
	if timeLimit <= 0 {
		timeLimit = 2000
	}
	if memoryLimit <= 0 {
		memoryLimit = 256
	}
	return timeLimit, memoryLimit
}

func (js *JudgeService) loadTestCaseIO(ctx context.Context, tc models.TestCase) (string, string, error) {
	if js.OSSClient == nil || js.OSSBucket == "" {
		return "", "", errors.New("oss is not configured")
	}
	if strings.TrimSpace(tc.InputKey) == "" || strings.TrimSpace(tc.OutputKey) == "" {
		return "", "", errors.New("testcase key is empty")
	}
	if tc.InputSize > 16*1024*1024 || tc.OutputSize > 16*1024*1024 {
		return "", "", errors.New("testcase file exceeds size limit")
	}
	input, err := js.OSSClient.GetObjectBytes(ctx, js.OSSBucket, tc.InputKey)
	if err != nil {
		return "", "", fmt.Errorf("read testcase input failed: %w", err)
	}
	output, err := js.OSSClient.GetObjectBytes(ctx, js.OSSBucket, tc.OutputKey)
	if err != nil {
		return "", "", fmt.Errorf("read testcase output failed: %w", err)
	}
	return string(input), string(output), nil
}

func normalizeOutput(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

func (js *JudgeService) DetectLanguage(code string) string { return js.detectLanguage(code) }

var javaDeclaration = regexp.MustCompile(`\b(?:public\s+)?class\s+[A-Za-z_$][A-Za-z0-9_$]*`)
var pythonStatement = regexp.MustCompile(`(?m)^\s*(?:from\s+\w|import\s+\w|def\s+\w|print\s*\(|[A-Za-z_].*=.*input\s*\()`)

func (js *JudgeService) detectLanguage(code string) string {
	if strings.Contains(code, "package main") || strings.Contains(code, "func main(") {
		return "go"
	}
	if javaDeclaration.MatchString(code) && (strings.Contains(code, "static void main") || strings.Contains(code, "import java.") || strings.Contains(code, "public class")) {
		return "java"
	}
	if strings.Contains(code, "#include") || strings.Contains(code, "int main(") || strings.Contains(code, "using namespace std") {
		return "cpp"
	}
	if pythonStatement.MatchString(code) || strings.Contains(code, "print(") || strings.Contains(code, "input(") || strings.Contains(code, "sys.stdin") {
		return "python"
	}
	return "go"
}

func (js *JudgeService) getTestCases(questionID int) ([]models.TestCase, error) {
	return js.getTestCasesContext(context.Background(), questionID)
}

func (js *JudgeService) getTestCasesContext(ctx context.Context, questionID int) ([]models.TestCase, error) {
	var cases []models.TestCase
	if err := js.DB.WithContext(ctx).Where("question_id = ?", questionID).Order("id ASC").Limit(1025).Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("query testcases failed: %w", err)
	}
	if len(cases) == 0 {
		return nil, errors.New("question has no available testcases")
	}
	if len(cases) > 1024 {
		return nil, errors.New("testcase count exceeds submission limit")
	}
	for _, tc := range cases {
		if strings.TrimSpace(tc.InputKey) == "" || strings.TrimSpace(tc.OutputKey) == "" {
			return nil, errors.New("testcase record is not oss-based")
		}
	}
	return cases, nil
}
