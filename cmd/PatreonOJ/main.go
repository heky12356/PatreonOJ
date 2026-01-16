package main

import (
	"context"
	"html/template"
	"log"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/graph"
	"dachuang/internal/models"
	"dachuang/internal/oss"
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

	// 初始化OJ首页数据
	if err := models.InitOjOverView(); err != nil {
		log.Fatalf("OJ首页数据初始化失败: %v", err)
	}

	// 初始化OSS客户端
	ossClient, err := oss.NewOSSClient(
		config.GlobalConfig.OSS.Address,
		config.GlobalConfig.OSS.PublicAddress,
		config.GlobalConfig.OSS.AccessKey,
		config.GlobalConfig.OSS.SecretKey,
	)
	if err != nil {
		log.Fatalf("OSS初始化失败: %v", err)
	} else {
		log.Printf("OSS初始化成功: %s", config.GlobalConfig.OSS.Address)
		// 自动创建存储桶
		ctx := context.Background()
		bucketName := config.GlobalConfig.OSS.BucketName
		if bucketName == "" {
			bucketName = "patreon-oj-cases" // 默认值
		}
		if err := ossClient.CreateBucket(ctx, bucketName); err != nil {
			log.Printf("Warning: 创建OSS存储桶失败: %v", err)
		} else {
			log.Printf("OSS存储桶检查/创建成功: %s", bucketName)
		}

		if len(config.GlobalConfig.OSS.PublicReadPrefixes) > 0 {
			if err := ossClient.SetPublicReadPrefixes(ctx, bucketName, config.GlobalConfig.OSS.PublicReadPrefixes); err != nil {
				log.Printf("Warning: 设置OSS桶公开读策略失败: %v", err)
			} else {
				log.Printf("OSS桶公开读策略设置成功: %v", config.GlobalConfig.OSS.PublicReadPrefixes)
			}
		}
	}

	var graphService *graph.QuestionGraphService
	if config.GlobalConfig != nil {
		neo4jClient, err := graph.NewNeo4jClient(graph.Neo4jConfig{
			URI:      config.GlobalConfig.GraphDatabase.Neo4j.URI,
			Username: config.GlobalConfig.GraphDatabase.Neo4j.Username,
			Password: config.GlobalConfig.GraphDatabase.Neo4j.Password,
			Database: config.GlobalConfig.GraphDatabase.Neo4j.Database,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize Neo4j client: %v", err)
		} else {
			graphService = graph.NewQuestionGraphService(neo4jClient)
			log.Printf("图数据库服务初始化成功: %s", config.GlobalConfig.GraphDatabase.Neo4j.URI)
		}
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
	routers.RoutersInit(r, ossClient, graphService)

	// 启动服务器
	serverAddr := config.GlobalConfig.GetServerAddr()
	log.Printf("服务器启动在端口: %s", serverAddr)
	if err := r.Run(serverAddr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
