package models

type Question struct {
	Id int `gorm:"primaryKey;autoIncrement" json:"id"`

	QuestionId string `gorm:"type:varchar(64);not null" json:"question_id"`

	Title      string `gorm:"not null" json:"title"`
	Content    string `gorm:"type:text" json:"content"`
	Difficulty string `json:"difficulty"`

	TimeLimit   int `gorm:"default:2000" json:"time_limit"`
	MemoryLimit int `gorm:"default:256" json:"memory_limit"`

	// Tags keeps comma-separated tags for compatibility with existing logic.
	Tags string `json:"tags"`

	// Optional normalized tag relation.
	TagID int   `json:"tag_id"`
	Tag   []Tag `gorm:"many2many:question_tag;" json:"tag"`

	Status string `gorm:"default:draft" json:"status"`
}

type TestCase struct {
	ID         uint `gorm:"primaryKey"`
	QuestionID int  `json:"question_id" gorm:"index"`

	// Backward-compatible inline testcase fields.
	Input          string `gorm:"type:text" json:"input"`
	ExpectedOutput string `gorm:"type:text" json:"expected_output"`

	// OSS 存储字段
	InputKey   string `json:"input_key" gorm:"type:varchar(255)"`  // 输入文件的 OSS 路径
	OutputKey  string `json:"output_key" gorm:"type:varchar(255)"` // 输出文件的 OSS 路径
	InputSize  int64  `json:"input_size"`                          // 输入文件大小 (字节)
	OutputSize int64  `json:"output_size"`                         // 输出文件大小 (字节)

	IsHidden bool `json:"is_hidden"` // 是否隐藏测试用例
}

func (Question) TableName() string {
	return "question"
}
