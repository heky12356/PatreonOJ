package models

import "time"

type UserSolvedQuestion struct {
	ID         uint      `gorm:"primaryKey"`
	UserUUID   string    `gorm:"size:36;not null;index:idx_user_question,unique;index"`
	QuestionID int       `gorm:"not null;index:idx_user_question,unique;index"` // 对应 question.id
	SolvedAt   time.Time `gorm:"not null;index"`
}

func (UserSolvedQuestion) TableName() string {
	return "user_solved_question"
}
