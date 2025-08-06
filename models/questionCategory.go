package models

type QuestionCategory struct {
    QuestionId int    `json:"question_id"`
    CategoryId int    `json:"category_id"`
}

func(QuestionCategory) TableName() string{
	return"question_category"
}