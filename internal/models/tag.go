package models

type Tag struct {
	ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"type:varchar(64);not null;uniqueIndex" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	Questions []Question `gorm:"many2many:question_tag;" json:"-"`
}

func (Tag) TableName() string {
	return "tag"
}
