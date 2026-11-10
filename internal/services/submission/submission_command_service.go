package submission

import (
	"encoding/json"
	"errors"
	"strings"

	"patreonoj/internal/models"
	judgesvc "patreonoj/internal/services/judge"

	"gorm.io/gorm"
)

var (
	ErrSubmissionUserNotFound = errors.New("user not found")
	ErrSubmissionUserDisabled = errors.New("user disabled")
	ErrSubmissionNotFound     = errors.New("submission not found")
)

type SubmissionCommandService struct {
	db           *gorm.DB
	judgeService *judgesvc.JudgeService
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
	var user models.User
	if err := s.db.Where("uuid = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionUserNotFound
		}
		return nil, err
	}
	if user.Status != "" && user.Status != "active" {
		return nil, ErrSubmissionUserDisabled
	}

	var question models.Question
	if err := s.db.Where("id = ?", questionPK).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionQuestionNotFound
		}
		return nil, err
	}

	lang := ""
	if s.judgeService != nil {
		lang = s.judgeService.DetectLanguage(code)
	}

	submission := models.NewSubmission(userID, question.Id, code, lang)
	if err := s.db.Create(submission).Error; err != nil {
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
	var question models.Question
	if err := s.db.Where("question_id = ?", strings.TrimSpace(questionID)).Order("id ASC").First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionQuestionNotFound
		}
		return nil, err
	}
	return s.CreateSubmission(userID, question.Id, code)
}

func (s *SubmissionCommandService) BuildSubmissionResult(submissionID string) (map[string]interface{}, error) {
	var submission models.Submission
	if err := s.db.Where("id = ?", submissionID).First(&submission).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}

	response := map[string]interface{}{
		"submission_id": submission.ID,
		"user_id":       submission.UserID,
		"question_pk":   submission.QuestionID,
		"status":        submission.Status,
		"created_at":    submission.CreatedAt,
		"updated_at":    submission.UpdatedAt,
	}

	var question models.Question
	if err := s.db.Where("id = ?", submission.QuestionID).First(&question).Error; err == nil {
		response["question_pk"] = question.Id
		response["question_id"] = question.QuestionId
	}

	switch submission.Status {
	case "completed":
		results := parseSubmissionResults(submission.Results)
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
