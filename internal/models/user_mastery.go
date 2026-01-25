package models

import (
	"time"
)

// UserSkillMastery 用户技能掌握度模型
// 存储用户对各个知识点(Skill)的掌握程度(0.0 - 1.0)
type UserSkillMastery struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    string    `gorm:"index:idx_user_skill,unique;not null;type:varchar(36)" json:"user_id"`
	SkillKey  string    `gorm:"type:varchar(255);index:idx_user_skill,unique;not null" json:"skill_key"`
	Mastery   float64   `gorm:"type:double;default:0" json:"mastery"` // 掌握度 0.0 - 1.0
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u *UserSkillMastery) TableName() string {
	return "user_skill_mastery"
}
