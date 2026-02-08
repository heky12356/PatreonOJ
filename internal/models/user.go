package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	Id          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Username    string `gorm:"type:varchar(64);uniqueIndex;not null" json:"username"`
	Password    string `gorm:"type:varchar(255);not null" json:"-"`
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
	Name        string `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	Permissions []Permission `gorm:"many2many:role_permission;" json:"permissions"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Permission 权限模型
type Permission struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string `gorm:"type:varchar(128);uniqueIndex;not null" json:"code"`
	Description string `gorm:"type:text" json:"description"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "user"
}
