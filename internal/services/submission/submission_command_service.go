package submission

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"

	"patreonoj/internal/models"
	judgesvc "patreonoj/internal/services/judge"
	"patreonoj/internal/util"

	"gorm.io/gorm"
)

var (
	ErrSubmissionUserNotFound      = errors.New("user not found")
	ErrSubmissionUserDisabled      = errors.New("user disabled")
	ErrSubmissionNotFound          = errors.New("submission not found")
	ErrSubmissionForbidden         = errors.New("submission access denied")
	ErrSubmissionCodeInvalid       = errors.New("code must contain between 1 and 262144 bytes")
	ErrSubmissionQueueFull         = errors.New("submission queue is full")
	ErrSubmissionQuestionAmbiguous = errors.New("question_id is ambiguous; use question_pk")
)

const MaxCodeBytes = 256 * 1024

type SubmissionCommandService struct {
	db           *gorm.DB
	judgeService *judgesvc.JudgeService
	createMu     sync.Mutex
}

func NewSubmissionCommandService(db *gorm.DB, judgeService *judgesvc.JudgeService) *SubmissionCommandService {
	return &SubmissionCommandService{
		db:           db,
		judgeService: judgeService,
	}
}

type SubmissionCreateResult struct {
	Submission *models.Submission
	QuestionPK int
	QuestionID string
}

func (s *SubmissionCommandService) CreateSubmission(userID string, questionPK int, code string) (*SubmissionCreateResult, error) {
	return s.CreateSubmissionContext(context.Background(), userID, questionPK, "", code, "", 0)
}

func (s *SubmissionCommandService) CreateSubmissionContext(ctx context.Context, userID string, questionPK int, questionID, code, language string, queueLimit int) (*SubmissionCreateResult, error) {
	if strings.TrimSpace(code) == "" || len(code) > MaxCodeBytes {
		return nil, ErrSubmissionCodeInvalid
	}
	s.createMu.Lock()
	defer s.createMu.Unlock()
	db := s.db.WithContext(ctx)
	var user models.User
	if err := db.Preload("Roles.Permissions").Where("uuid = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionUserNotFound
		}
		return nil, err
	}
	if user.Status != "" && user.Status != "active" {
		return nil, ErrSubmissionUserDisabled
	}

	var question models.Question
	query := db.Where("id = ?", questionPK)
	if questionPK <= 0 {
		var matches []models.Question
		if err := db.Where("question_id = ?", strings.TrimSpace(questionID)).Order("id ASC").Limit(2).Find(&matches).Error; err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, ErrSubmissionQuestionNotFound
		}
		if len(matches) != 1 {
			return nil, ErrSubmissionQuestionAmbiguous
		}
		query = db.Where("id = ?", matches[0].Id)
	}
	if err := query.First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionQuestionNotFound
		}
		return nil, err
	}
	if question.Status != "published" && !util.UserInstance.HasPermission(user, "admin") {
		return nil, ErrSubmissionForbidden
	}
	if queueLimit > 0 {
		var pending int64
		if err := db.Model(&models.Submission{}).Where("status = ?", "pending").Count(&pending).Error; err != nil {
			return nil, err
		}
		if pending >= int64(queueLimit) {
			return nil, ErrSubmissionQueueFull
		}
	}

	lang := language
	if lang == "" && s.judgeService != nil {
		lang = s.judgeService.DetectLanguage(code)
	}

	submission := models.NewSubmission(userID, question.Id, code, lang)
	if err := db.Create(submission).Error; err != nil {
		return nil, err
	}

	return &SubmissionCreateResult{
		Submission: submission,
		QuestionPK: question.Id,
		QuestionID: question.QuestionId,
	}, nil
}

func (s *SubmissionCommandService) CreateSubmissionByQuestionNumber(userID string, questionNumber int, code string) (*SubmissionCreateResult, error) {
	return s.CreateSubmission(userID, questionNumber, code)
}

func (s *SubmissionCommandService) CreateSubmissionByQuestionID(userID string, questionID string, code string) (*SubmissionCreateResult, error) {
	return s.CreateSubmissionContext(context.Background(), userID, 0, questionID, code, "", 0)
}

func (s *SubmissionCommandService) BuildSubmissionResult(submissionID string) (map[string]interface{}, error) {
	return s.buildSubmissionResult(submissionID, nil)
}

func (s *SubmissionCommandService) BuildSubmissionResultForViewer(submissionID string, viewer models.User) (map[string]interface{}, error) {
	return s.buildSubmissionResult(submissionID, &viewer)
}

func (s *SubmissionCommandService) buildSubmissionResult(submissionID string, viewer *models.User) (map[string]interface{}, error) {
	var submission models.Submission
	if err := s.db.Where("id = ?", submissionID).First(&submission).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	if viewer != nil && viewer.UUID != submission.UserID && !util.UserInstance.HasPermission(*viewer, "admin") && !submission.IsPublic {
		return nil, ErrSubmissionForbidden
	}

	var author models.User
	if err := s.db.Select("id", "uuid", "username", "show_activity").Where("uuid = ?", submission.UserID).First(&author).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if viewer != nil && viewer.UUID != submission.UserID && !util.UserInstance.HasPermission(*viewer, "admin") && author.UUID != "" && !author.ShowActivity {
		return nil, ErrSubmissionForbidden
	}
	response := map[string]interface{}{
		"verdict":       submission.ResultVerdict(),
		"username":      author.Username,
		"code":          submission.Code,
		"language":      submission.Language,
		"runtime_ms":    submission.RuntimeMs,
		"memory_kb":     submission.MemoryKB,
		"code_length":   submission.CodeLength,
		"submission_id": submission.ID,
		"user_id":       submission.UserID,
		"question_pk":   submission.QuestionID,
		"status":        submission.Status,
		"created_at":    submission.CreatedAt,
		"updated_at":    submission.UpdatedAt,
	}

	var question models.Question
	if err := s.db.Where("id = ?", submission.QuestionID).First(&question).Error; err == nil {
		if viewer != nil && question.Status != "published" && !util.UserInstance.HasPermission(*viewer, "admin") {
			return nil, ErrSubmissionForbidden
		}
		response["question_pk"] = question.Id
		response["question_id"] = question.QuestionId
		response["question_title"] = question.Title
	}

	switch submission.Status {
	case "completed":
		results := parseSubmissionResults(submission.Results)
		// Visibility may have changed since judging. Missing/deleted cases and
		// historical records without an ID are private by default.
		caseIDs := make([]uint, 0, len(results))
		for _, result := range results {
			if result.TestCaseID != 0 {
				caseIDs = append(caseIDs, result.TestCaseID)
			}
		}
		visible := make(map[uint]bool)
		for start := 0; start < len(caseIDs); start += 500 {
			var cases []models.TestCase
			if err := s.db.Select("id", "is_hidden").Where("question_id = ? AND id IN ?", submission.QuestionID, caseIDs[start:min(start+500, len(caseIDs))]).Find(&cases).Error; err != nil {
				return nil, err
			}
			for _, testcase := range cases {
				visible[testcase.ID] = !testcase.IsHidden
			}
		}
		for i := range results {
			results[i].Verdict = results[i].ResultVerdict()
			if results[i].Hidden || !visible[results[i].TestCaseID] {
				results[i].Hidden = true
				results[i].Input, results[i].ExpectedOutput, results[i].ActualOutput = "", "", ""
			}
		}
		if len(results) == 0 {
			response["results"] = []models.TestCaseResult{}
			response["pass_rate"] = 0.0
			response["total_cases"] = 0
			response["passed_cases"] = 0
			response["message"] = "评测完成，但未返回测试用例结果"
			return response, nil
		}

		passed := 0
		for _, result := range results {
			if result.IsCorrect {
				passed++
			}
		}
		response["results"] = results
		response["pass_rate"] = float64(passed) / float64(len(results))
		response["total_cases"] = len(results)
		response["passed_cases"] = passed
	case "pending":
		response["results"] = []models.TestCaseResult{}
		response["message"] = "代码已进入评测队列，请稍后重试"
	case "processing":
		response["results"] = []models.TestCaseResult{}
		response["message"] = "代码正在评测中，请稍后重试"
	case "error":
		response["results"] = []models.TestCaseResult{}
		response["error_code"] = submission.ErrorCode
		response["error_message"] = submission.ErrorMsg
		response["message"] = friendlySubmissionErrorMessage(submission.ErrorCode)
	default:
		response["results"] = []models.TestCaseResult{}
		response["message"] = "未知提交状态"
	}

	return response, nil
}

func ParseSubmissionResults(resultsStr string) []models.TestCaseResult {
	return parseSubmissionResults(resultsStr)
}

func parseSubmissionResults(resultsStr string) []models.TestCaseResult {
	if resultsStr == "" {
		return []models.TestCaseResult{}
	}

	var results []models.TestCaseResult
	if err := json.Unmarshal([]byte(resultsStr), &results); err != nil {
		return []models.TestCaseResult{}
	}
	return results
}

func friendlySubmissionErrorMessage(errorCode string) string {
	switch errorCode {
	case "E001":
		return "题目配置异常：缺少可用测试用例"
	case "E002":
		return "编译失败，请检查代码语法"
	case "E003":
		return "超出时间限制，请优化算法"
	case "E004":
		return "超出内存限制，请优化内存使用"
	case "E005":
		return "评测服务不可用，请稍后重试"
	case "E999":
		return "系统内部错误"
	default:
		return "未知错误"
	}
}

func EnsureSubmissionErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var netErr net.Error
	if errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) {
		return "E005"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case containsAny(msg,
		"test case", "testcase", "testcases",
		"question has no available testcases",
		"missing test cases", "testcase key is empty",
		"testcase record is not oss-based",
		"oss is not configured",
		"read testcase input failed",
		"read testcase output failed",
	):
		return "E001"
	case containsAny(msg, "compile", "compilation failed", "compile error"):
		return "E002"
	case containsAny(msg, "timeout", "time limit exceeded", "time limit"):
		return "E003"
	case containsAny(msg, "memory limit exceeded", "memory exceeded", "out of memory", "memory"):
		return "E004"
	case containsAny(msg,
		"network", "dial tcp", "connection refused", "no such host",
		"go-judge api error", "bad gateway", "service unavailable",
		"404", "502", "503", "504",
	):
		return "E005"
	default:
		return "E999"
	}
}

func containsAny(msg string, terms ...string) bool {
	for _, t := range terms {
		if strings.Contains(msg, t) {
			return true
		}
	}
	return false
}
