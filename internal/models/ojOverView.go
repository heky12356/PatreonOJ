package models

type OjOverView struct {
	Id             int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TotalProlemCnt int    `json:"total_problem_cnt"`
	HomeText       string `json:"home_text"`
}

// InitOjOverView 初始化OJ首页数据
func InitOjOverView() error {
	// 检查是否已存在数据
	var count int64
	DB.Model(&OjOverView{}).Count(&count)
	if count > 0 {
		return nil // 已存在数据，无需初始化
	}

	// 创建默认记录
	defaultView := &OjOverView{
		TotalProlemCnt: 0,
		HomeText:       "欢迎来到PatreonOJ",
	}
	return DB.Create(defaultView).Error
}
