package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"dachuang/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() error {
	var err error
	var dialector gorm.Dialector

	// 根据配置选择数据库类型
	switch config.GlobalConfig.Database.Type {
	case "mysql":
		dsn := config.GlobalConfig.GetDatabaseDSN()
		dialector = mysql.Open(dsn)
		log.Printf("使用MySQL数据库，连接字符串: %s", maskPassword(dsn))
	case "sqlite":
		dbPath := config.GlobalConfig.GetDatabaseDSN()
		// 确保SQLite数据库目录存在
		if err = ensureDir(filepath.Dir(dbPath)); err != nil {
			return fmt.Errorf("创建SQLite数据库目录失败: %w", err)
		}
		dialector = sqlite.Open(dbPath)
		log.Printf("使用SQLite数据库，路径: %s", dbPath)
	default:
		return fmt.Errorf("不支持的数据库类型: %s", config.GlobalConfig.Database.Type)
	}

	// 配置GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// 建立数据库连接
	DB, err = gorm.Open(dialector, gormConfig)
	if err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 配置连接池（仅对MySQL有效）
	if config.GlobalConfig.Database.Type == "mysql" {
		sqlDB, err := DB.DB()
		if err != nil {
			return fmt.Errorf("获取数据库实例失败: %w", err)
		}

		// 设置连接池参数
		sqlDB.SetMaxIdleConns(10)      // 最大空闲连接数
		sqlDB.SetMaxOpenConns(100)     // 最大打开连接数
		sqlDB.SetConnMaxLifetime(3600) // 连接最大生存时间（秒）
	}

	log.Println("数据库连接成功")
	return nil
}

// ensureDir 确保目录存在
func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// maskPassword 隐藏连接字符串中的密码
func maskPassword(dsn string) string {
	// 简单的密码隐藏逻辑，实际项目中可以使用更复杂的方法
	return "***masked***"
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	// 自动迁移所有模型
	err := DB.AutoMigrate(
		&User{},
		&Role{},
		&Permission{},
		&UserQuestionMastery{},
		&UserTagMastery{},
		&Question{},
		&Category{},
		&Submission{},
		&TestCase{},
		&Relation{},
		&Node{},
		&QuestionCategory{},

		&UserSolve{},
		&UserSkillMastery{},
		&OjOverView{},
		// 如果有其他模型，在这里添加
	)
	if err != nil {
		return fmt.Errorf("数据库表迁移失败: %w", err)
	}

	log.Println("数据库表迁移完成")
	return nil
}
