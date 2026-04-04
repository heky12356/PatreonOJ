package services

import (
	"encoding/json"
	"errors"
	"strings"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var (
	ErrSubmissionUserNotFound = errors.New("user not found")
	ErrSubmissionUserDisabled = errors.New("user disabled")
	ErrSubmissionNotFound     = errors.New("submission not found")
)

type SubmissionCommandService struct {
	db           *gorm.DB
	judgeService *JudgeService
}

func NewSubmissionCommandService(db *gorm.DB, judgeService *JudgeService) *SubmissionCommandService {
	return &SubmissionCommandService{
		db:           db,
		judgeService: judgeService,
	}
}

type SubmissionCreateResult struct {
	Submission     *models.Submission
	QuestionNumber int
}

func (s *SubmissionCommandService) CreateSubmission(userID string, questionNumber int, code string) (*SubmissionCreateResult, error) {
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
	if err := s.db.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
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
		Submission:     submission,
		QuestionNumber: questionNumber,
	}, nil
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
		"question_id":   submission.QuestionID,
		"status":        submission.Status,
		"created_at":    submission.CreatedAt,
		"updated_at":    submission.UpdatedAt,
	}

	var question models.Question
	if err := s.db.Where("id = ?", submission.QuestionID).First(&question).Error; err == nil {
		response["question_number"] = question.QuestionNumber
	}

	switch submission.Status {
	case "completed":
		results := parseSubmissionResults(submission.Results)
		if len(results) == 0 {
			response["results"] = []models.TestCaseResult{}
			response["pass_rate"] = 0.0
			response["total_cases"] = 0
			response["passed_cases"] = 0
			response["message"] = "评测完成但无测试结果"
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
		response["message"] = "代码正在评测中，请稍后查询"
	case "error":
		response["results"] = []models.TestCaseResult{}
		response["error_code"] = submission.ErrorCode
		response["error_message"] = submission.ErrorMsg
		response["message"] = friendlySubmissionErrorMessage(submission.ErrorCode)
	default:
		response["results"] = []models.TestCaseResult{}
		response["message"] = "代码已提交，等待评测"
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
		return "题目配置错误：缺少测试用例，请联系管理员"
	case "E002":
		return "代码编译失败，请检查语法错误"
	case "E003":
		return "代码运行超时，请优化算法效率"
	case "E004":
		return "内存使用超限，请优化内存使用"
	case "E005":
		return "网络连接错误，请稍后重试"
	case "E999":
		return "系统内部错误，请联系管理员"
	default:
		return "未知错误，请联系管理员"
	}
}

func EnsureSubmissionErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "测试用例"), strings.Contains(msg, "test case"):
		return "E001"
	case strings.Contains(msg, "编译"), strings.Contains(msg, "compile"):
		return "E002"
	case strings.Contains(msg, "超时"), strings.Contains(msg, "timeout"), strings.Contains(msg, "time limit"):
		return "E003"
	case strings.Contains(msg, "内存"), strings.Contains(msg, "memory"):
		return "E004"
	case strings.Contains(msg, "网络"), strings.Contains(msg, "network"):
		return "E005"
	default:
		return "E999"
	}
}
