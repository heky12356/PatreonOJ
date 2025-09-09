package models

type User struct {
	Id          int
	Username    string
	Password    string `gorm:"not null"`
	UUID        string `gorm:"size:36"`
	Permissions string
}

func (User) TableName() string {
	return "user"
}
