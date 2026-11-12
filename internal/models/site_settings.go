package models

import "time"

// SiteSettings is a singleton. Theme selection and carousel content are independent.
type SiteSettings struct {
	ID                 uint   `gorm:"primaryKey"`
	ThemeID            string `gorm:"size:64;not null"`
	CarouselEnabled    bool   `gorm:"not null"`
	CarouselIntervalMS int    `gorm:"not null"`
	SlidesJSON         string `gorm:"type:text;not null"`
	UpdatedAt          time.Time
}
