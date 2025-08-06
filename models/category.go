package  models

type Category struct {
    
	Id int
	Name string
	Question []Question `gorm:"many2many:question_category;"`
}

func(Category) TableName() string{
	return "category"
}