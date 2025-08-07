package models

import(
 "github.com/google/uuid"
	 "time"
   
)

type Submission struct {
    ID         string    `json:"id" gorm:"primaryKey"`
    UserID     string    `json:"user_id"`
    QuestionID string    `json:"question_id"`
    Code       string    `json:"code" gorm:"type:text"`
    Status     string    `json:"status"`
    Results    []TestCaseResult `json:"results" gorm:"type:json"`
    CreatedAt  time.Time `json:"created_at"`  // 标准GORM创建时间字段
    UpdatedAt  time.Time `json:"updated_at"`  // 标准GORM更新时间字段
    // 如果你的业务需要额外字段，可以添加:
    // CreateAkt  string `json:"create_akt"`
    // UpdatedAkt string `json:"updated_akt"`
}
type TestCaseResult struct {
    Input          string `json:"input"`
    ExpectedOutput string `json:"expected_output"`
    ActualOutput   string `json:"actual_output"`
    IsCorrect      bool   `json:"is_correct"`
    Runtime        int64  `json:"runtime"` // 毫秒
    MemoryUsage    int64  `json:"memory_usage"` // KB
}

func NewSubmission(userID, questionID, code string) *Submission {  // 参数名改为questionID
    return &Submission{
        ID:        uuid.New().String(),
        UserID:    userID,
        QuestionID: questionID,  // 改为QuestionID
        Code:      code,
        Status:    "pending",
    }
}