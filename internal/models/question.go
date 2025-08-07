package  models

type Question struct {
    // 数据库主键ID（自增）
	Id int  `gorm:"primaryKey;autoIncrement" json:"id"`
	
	// 题目编号（从1001开始，用于显示）
	QuestionNumber int `gorm:"unique;not null" json:"question_number"`
	
	// 基础信息
	Title string `gorm:"not null" json:"title"`                    // 题目标题
	Content string `gorm:"type:text" json:"content"`               // 题目描述
	Difficulty string `json:"difficulty"`                          // 难度等级
	
	// 格式说明
	InputFormat string `gorm:"type:text" json:"input_format"`      // 输入格式说明
	OutputFormat string `gorm:"type:text" json:"output_format"`    // 输出格式说明
	
	// 样例数据
	SampleInput string `gorm:"type:text" json:"sample_input"`      // 样例输入
	SampleOutput string `gorm:"type:text" json:"sample_output"`    // 样例输出
	SampleExplanation string `gorm:"type:text" json:"sample_explanation"` // 样例解释
	
	// 约束条件
	DataRange string `gorm:"type:text" json:"data_range"`          // 数据范围
	TimeLimit int `gorm:"default:2000" json:"time_limit"`          // 时间限制（毫秒）
	MemoryLimit int `gorm:"default:256" json:"memory_limit"`       // 内存限制（MB）
	
	// 元数据
	Source string `json:"source"`                                  // 题目来源
	Tags string `json:"tags"`                                      // 题目标签（逗号分隔）
	Hint string `gorm:"type:text" json:"hint"`                     // 提示信息
	
	// 分类关联
	Category_id int
	Category []Category `gorm:"many2many:question_category;"`
	
	// 状态信息
	Status string `gorm:"default:draft" json:"status"`             // 题目状态：draft/published/archived
}
type TestCase struct {
    ID             uint   `gorm:"primaryKey"`
    QuestionID     int    `json:"question_id"`  // 改为int类型，与Question.Id匹配
    Input          string `gorm:"type:text"`
    ExpectedOutput string `gorm:"type:text"`
    IsHidden       bool   // 是否隐藏测试用例
}
func(Question) TableName() string{
	return "question"
}