package models

type QuestionTag struct {
	QuestionId int `gorm:"primaryKey" json:"question_id"`
	TagId      int `gorm:"primaryKey" json:"tag_id"`
}

func (QuestionTag) TableName() string {
	return "question_tag"
}
