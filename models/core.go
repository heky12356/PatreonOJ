package models

import(
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	
)

var DB *gorm.DB
var err error
func init(){
	dsn := "root:zzx123.1@tcp(localhost:3306)/ye?charset=utf8mb4&parseTime=True&loc=Local"
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("数据库连接失败")
	}
}