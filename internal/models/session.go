package models

import "time"

type Session struct {
	Id        int        `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionId string     `gorm:"unique" json:"session_id"`
	UserId    int        `json:"user_id"`
	ExpiresAt *time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
