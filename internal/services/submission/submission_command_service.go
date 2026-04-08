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
			response["message"] = "璇勬祴瀹屾垚浣嗘棤娴嬭瘯缁撴灉"
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
		response["message"] = "code is being judged, please retry later"
	case "error":
		response["results"] = []models.TestCaseResult{}
		response["error_code"] = submission.ErrorCode
		response["error_message"] = submission.ErrorMsg
		response["message"] = friendlySubmissionErrorMessage(submission.ErrorCode)
	default:
		response["results"] = []models.TestCaseResult{}
		response["message"] = "浠ｇ爜宸叉彁浜わ紝绛夊緟璇勬祴"
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
		return "question configuration error: missing test cases"
	case "E002":
		return "compilation failed, please check syntax"
	case "E003":
		return "time limit exceeded, please optimize algorithm"
	case "E004":
		return "memory limit exceeded, please optimize memory usage"
	case "E005":
		return "network error, please retry later"
	case "E999":
		return "internal system error"
	default:
		return "unknown error"
	}
}

func EnsureSubmissionErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "娴嬭瘯鐢ㄤ緥"), strings.Contains(msg, "test case"):
		return "E001"
	case strings.Contains(msg, "缂栬瘧"), strings.Contains(msg, "compile"):
		return "E002"
	case strings.Contains(msg, "瓒呮椂"), strings.Contains(msg, "timeout"), strings.Contains(msg, "time limit"):
		return "E003"
	case strings.Contains(msg, "鍐呭瓨"), strings.Contains(msg, "memory"):
		return "E004"
	case strings.Contains(msg, "缃戠粶"), strings.Contains(msg, "network"):
		return "E005"
	default:
		return "E999"
	}
}
