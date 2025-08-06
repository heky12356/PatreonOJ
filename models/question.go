package  models

type Question struct {
    
	Id int  `gorm:"primaryKey;autoIncrement" json:"id"`
	Title string
	Content string
	Difficulty string
	Category_id int
	Category []Category `gorm:"many2many:question_category;"`


}
type TestCase struct {
    ID             uint   `gorm:"primaryKey"`
    QuestionID     uint   // 关联到题目
    Input          string `gorm:"type:text"`
    ExpectedOutput string `gorm:"type:text"`
    IsHidden       bool   // 是否隐藏测试用例
}
func(Question) TableName() string{
	return "question"
}