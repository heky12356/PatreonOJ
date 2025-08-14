package models

import (
	"time"
)

type UserSolve struct {
	ID         uint `gorm:"primarykey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  time.Time `gorm:"index"`
	UUID       string    `gorm:"type:varchar(255);not null"`
	ProblemIDs string    `gorm:"type:varchar(255);null"`
}

func (u *UserSolve) TableName() string {
	return "user_solve"
}
