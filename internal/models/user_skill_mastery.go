package models

import "time"

// UserSkillMastery stores a user's mastery score for one skill/tag.
type UserSkillMastery struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   string `gorm:"type:varchar(64);not null;uniqueIndex:idx_user_skill_mastery" json:"user_id"`
	SkillKey string `gorm:"type:varchar(128);not null;uniqueIndex:idx_user_skill_mastery;index" json:"skill_key"`

	// Mastery is normalized to [0, 1].
	Mastery float64 `gorm:"not null;default:0" json:"mastery"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserSkillMastery) TableName() string {
	return "user_skill_mastery"
}
