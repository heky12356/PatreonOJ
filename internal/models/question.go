package models

type Question struct {
	// 数据库主键ID（自增）
	Id int `gorm:"primaryKey;autoIncrement" json:"id"`

	// 题目编号（从1001开始，用于显示）
	QuestionNumber int `gorm:"unique;not null" json:"question_number"`

	// 题目id（自定义）
	QuestionId string `gorm:"type:text" json:"question_id"`

	// 基础信息
	Title      string `gorm:"not null" json:"title"`    // 题目标题
	Content    string `gorm:"type:text" json:"content"` // 题目描述
	Difficulty string `json:"difficulty"`               // 难度等级

	// 约束条件
	DataRange   string `gorm:"type:text" json:"data_range"`     // 数据范围
	TimeLimit   int    `gorm:"default:2000" json:"time_limit"`  // 时间限制（毫秒）
	MemoryLimit int    `gorm:"default:256" json:"memory_limit"` // 内存限制（MB）

	// 元数据
	Tags string `json:"tags"` // 题目标签（逗号分隔）

	// 分类关联
	Category_id int
	Category    []Category `gorm:"many2many:question_category;"`

	// 状态信息
	Status string `gorm:"default:draft" json:"status"` // 题目状态：draft/published/archived/hidden
}
type TestCase struct {
	ID         uint `gorm:"primaryKey"`
	QuestionID int  `json:"question_id" gorm:"index"` // 改为int类型，与Question.Id匹配

	// 兼容旧的小文本存储（可选）
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
