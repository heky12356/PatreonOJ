package models

import(
 "github.com/google/uuid"
	 "time"
   
)

type Submission struct {
    ID         string    `json:"id" gorm:"primaryKey"`
    UserID     string    `json:"user_id" gorm:"index"`
    QuestionID int       `json:"question_id" gorm:"index"`  // 改为int类型，存储题目的数据库ID

    Language   string    `json:"language" gorm:"type:varchar(32);index"`
    CodeLength int       `json:"code_length" gorm:"index"`

    RuntimeMs int64 `json:"runtime_ms"`
    MemoryKB  int64 `json:"memory_kb"`

    IsPublic bool `json:"is_public" gorm:"default:true;index"`

    Code      string `json:"code" gorm:"type:text"`
    Status    string `json:"status"`
    Results   string `json:"results" gorm:"type:text"` // 改为string类型，存储JSON字符串
    ErrorCode string `json:"error_code"`                // 错误码
    ErrorMsg  string `json:"error_msg"`                 // 错误信息

    CreatedAt time.Time `json:"created_at"` // 标准GORM创建时间字段
    UpdatedAt time.Time `json:"updated_at"` // 标准GORM更新时间字段
}

type TestCaseResult struct {
    Input          string `json:"input"`
    ExpectedOutput string `json:"expected_output"`
    ActualOutput   string `json:"actual_output"`
    IsCorrect      bool   `json:"is_correct"`
    Runtime        int64  `json:"runtime"` // 毫秒
    MemoryUsage    int64  `json:"memory_usage"` // KB
}

func NewSubmission(userID string, questionID int, code string, language string) *Submission {
    return &Submission{
        ID:         uuid.New().String(),
        UserID:     userID,
        QuestionID: questionID, // 现在接收int类型的题目ID

        Language:   language,
        CodeLength: len(code),
        RuntimeMs:  0,
        MemoryKB:   0,
        IsPublic:   true,

        Code:     code,
        Status:   "pending",
        Results:  "", // 初始化为空字符串
        ErrorCode: "", // 初始化为空
        ErrorMsg:  "", // 初始化为空
    }
}