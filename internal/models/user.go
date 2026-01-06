package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	Id          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Username    string `gorm:"uniqueIndex;not null" json:"username"`
	Password    string `gorm:"not null" json:"-"`
	UUID        string `gorm:"size:36;uniqueIndex;not null" json:"uuid"`
	Nickname    string `gorm:"type:varchar(64)" json:"nickname"`
	Email       string `gorm:"type:varchar(255)" json:"email"`
	AvatarURL   string `gorm:"type:text" json:"avatar_url"`
	Permissions string `gorm:"type:text" json:"permissions"`
	Status      string `gorm:"type:varchar(32);default:active;index" json:"status"`

	Roles []Role `gorm:"many2many:user_role;" json:"roles"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Role 角色模型
type Role struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	Permissions []Permission `gorm:"many2many:role_permission;" json:"permissions"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Permission 权限模型
type Permission struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string `gorm:"uniqueIndex;not null" json:"code"`
	Description string `gorm:"type:text" json:"description"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserQuestionMastery 用户题目掌握模型
type UserQuestionMastery struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserUUID       string `gorm:"size:36;not null;index:idx_user_question,unique" json:"user_uuid"`
	QuestionNumber int    `gorm:"not null;index:idx_user_question,unique;index" json:"question_number"`

	// 统计信息
	Attempts      int     `gorm:"not null;default:0" json:"attempts"`       // 总提交次数
	AcceptedCount int     `gorm:"not null;default:0" json:"accepted_count"` // 已AC次数
	Mastery       float64 `gorm:"not null;default:0" json:"mastery"`        // 掌握度，0-1之间的浮点数

	// 最后提交时间
	LastSubmittedAt *time.Time `json:"last_submitted_at"`
	LastAcceptedAt  *time.Time `json:"last_accepted_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserTagMastery 用户标签掌握模型
type UserTagMastery struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserUUID string `gorm:"size:36;not null;index:idx_user_tag,unique" json:"user_uuid"`
	Tag      string `gorm:"type:varchar(128);not null;index:idx_user_tag,unique;index" json:"tag"`

	// 统计信息
	Attempts      int     `gorm:"not null;default:0" json:"attempts"`       // 总提交次数
	AcceptedCount int     `gorm:"not null;default:0" json:"accepted_count"` // 已AC次数
	Mastery       float64 `gorm:"not null;default:0" json:"mastery"`        // 掌握度，0-1之间的浮点数

	// 最后提交时间
	LastSubmittedAt *time.Time `json:"last_submitted_at"`
	LastAcceptedAt  *time.Time `json:"last_accepted_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "user"
}
