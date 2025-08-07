package models

import(
 "github.com/google/uuid"
	 "time"
   
)

type Submission struct {
    ID         string    `json:"id" gorm:"primaryKey"`
    UserID     string    `json:"user_id"`
    QuestionID int       `json:"question_id"`  // 改为int类型，存储题目的数据库ID
    Code       string    `json:"code" gorm:"type:text"`
    Status     string    `json:"status"`
    Results    []TestCaseResult `json:"results" gorm:"type:json"`
    CreatedAt  time.Time `json:"created_at"`  // 标准GORM创建时间字段
    UpdatedAt  time.Time `json:"updated_at"`  // 标准GORM更新时间字段
}
type TestCaseResult struct {
    Input          string `json:"input"`
    ExpectedOutput string `json:"expected_output"`
    ActualOutput   string `json:"actual_output"`
    IsCorrect      bool   `json:"is_correct"`
    Runtime        int64  `json:"runtime"` // 毫秒
    MemoryUsage    int64  `json:"memory_usage"` // KB
}

func NewSubmission(userID string, questionID int, code string) *Submission {
    return &Submission{
        ID:         uuid.New().String(),
        UserID:     userID,
        QuestionID: questionID,  // 现在接收int类型的题目ID
        Code:       code,
        Status:     "pending",
    }
}