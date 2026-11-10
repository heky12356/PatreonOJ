package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"patreonoj/internal/config"

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
		log.Printf("使用MySQL数据库: %s:%d/%s", config.GlobalConfig.Database.MySQL.Host, config.GlobalConfig.Database.MySQL.Port, config.GlobalConfig.Database.MySQL.DBName)
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
		Logger: logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
			SlowThreshold:        time.Second,
			LogLevel:             logger.Warn,
			ParameterizedQueries: true,
		}),
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
		sqlDB.SetMaxIdleConns(10)  // 最大空闲连接数
		sqlDB.SetMaxOpenConns(100) // 最大打开连接数
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		sqlDB, err := DB.DB()
		if err != nil {
			return fmt.Errorf("获取数据库实例失败: %w", err)
		}
		sqlDB.SetMaxOpenConns(1)
		if err := DB.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
			return err
		}
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
		&Question{},
		&Tag{},
		&Submission{},
		&TestCase{},
		&Relation{},
		&QuestionRelationEdge{},
		&Node{},
		&QuestionTag{},
		&UserSolvedQuestion{},
		&UserSkillMastery{},
		&OjOverView{},
		&Session{},
		// 如果有其他模型，在这里添加
	)
	if err != nil {
		return fmt.Errorf("数据库表迁移失败: %w", err)
	}

	log.Println("数据库表迁移完成")
	if err := EnsureBuiltinRoles(DB); err != nil {
		return fmt.Errorf("初始化基础角色失败: %w", err)
	}
	return BackfillSubmissionVerdicts()
}
