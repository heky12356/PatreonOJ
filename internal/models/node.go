package  models

type Node struct {
    
	  ID      int    `gorm:"primaryKey;autoIncrement" json:"id"`
    Name    string `gorm:"type:varchar(255);not null" json:"name"`
    Type    string `gorm:"type:varchar(255);not null" json:"type"`
    Content string `gorm:"type:text" json:"content"`
	
}

func(Node) TableName() string{
	return "Node"
}