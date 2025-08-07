package main

import (
	"html/template"
	"log"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/models"
	"dachuang/internal/routers"

	"github.com/gin-gonic/gin"
)

// UserInfo 结构体定义（当前未使用）
type UserInfo struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

// UnixToTime 将Unix时间戳转换为格式化的时间字符串
func UnixToTime(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}

func main() {
	// 初始化配置
	configPath := "config.yaml"
	if err := config.InitConfig(configPath); err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}

	// 初始化数据库
	if err := models.InitDB(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 自动迁移数据库表
	if err := models.AutoMigrate(); err != nil {
		log.Fatalf("数据库表迁移失败: %v", err)
	}

	// 设置Gin运行模式
	gin.SetMode(config.GlobalConfig.Server.Mode)

	// 创建路由引擎
	r := gin.Default()

	// 自定义模板函数 - 注意要放在加载模板前面
	r.SetFuncMap(template.FuncMap{
		"UnixToTime": UnixToTime,
	})

	// 初始化路由
	routers.RoutersInit(r)

	// 启动服务器
	serverAddr := config.GlobalConfig.GetServerAddr()
	log.Printf("服务器启动在端口: %s", serverAddr)
	if err := r.Run(serverAddr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
