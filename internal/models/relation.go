package models



// Relation 对应数据库中的 relation 表
type Relation struct {
    ID        int    `gorm:"primaryKey;autoIncrement" json:"id"`
    SourceID  int    `gorm:"type:int;not null" json:"source_id"`
    TargetID  int    `gorm:"type:int;not null" json:"target_id"`
    Relation  string `gorm:"type:varchar(255);not null" json:"relation"`
}

// 初始化表结构
func (Relation) TableName() string {
    return "relation"
}