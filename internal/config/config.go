package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// Config 全局配置结构体
type Config struct {
	Database      DatabaseConfig      `mapstructure:"database"`
	GraphDatabase GraphDatabaseConfig `mapstructure:"graph_database"`
	Server        ServerConfig        `mapstructure:"server"`
	Judge         JudgeConfig         `mapstructure:"judge"`
	Log           LogConfig           `mapstructure:"log"`
	OSS           OSSConfig           `mapstructure:"oss"`
}

// OSSConfig OSS配置
type OSSConfig struct {
	Address    string `mapstructure:"address"`
	AccessKey  string `mapstructure:"access_key"`
	SecretKey  string `mapstructure:"secret_key"`
	BucketName string `mapstructure:"bucket_name"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type   string       `mapstructure:"type"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	SQLite SQLiteConfig `mapstructure:"sqlite"`
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	DBName    string `mapstructure:"dbname"`
	Charset   string `mapstructure:"charset"`
	ParseTime bool   `mapstructure:"parseTime"`
	Loc       string `mapstructure:"loc"`
}

// SQLiteConfig SQLite配置
type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

// GraphDatabaseConfig 图数据库配置
type GraphDatabaseConfig struct {
	Neo4j Neo4jConfig `mapstructure:"neo4j"`
}

// Neo4jConfig Neo4j配置
type Neo4jConfig struct {
	URI      string `mapstructure:"uri"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// JudgeConfig 评测服务配置
type JudgeConfig struct {
	Mode      string           `mapstructure:"mode"`
	APIURL    string           `mapstructure:"api_url"`
	Timeout   int              `mapstructure:"timeout"`
	QueueSize int              `mapstructure:"queue_size"`
	Local     LocalJudgeConfig `mapstructure:"local"`
}

// LocalJudgeConfig 本地评测配置
type LocalJudgeConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	SandboxDir         string   `mapstructure:"sandbox_dir"`
	MaxMemory          int      `mapstructure:"max_memory"`
	MaxTime            int      `mapstructure:"max_time"`
	MaxOutputSize      int      `mapstructure:"max_output_size"`
	SupportedLanguages []string `mapstructure:"supported_languages"`

	Executor string `mapstructure:"executor"` // host/docker

	DockerImageGo     string `mapstructure:"docker_image_go"`
	DockerImageCpp    string `mapstructure:"docker_image_cpp"`
	DockerImagePython string `mapstructure:"docker_image_python"`
	DockerImageJava   string `mapstructure:"docker_image_java"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	Output   string `mapstructure:"output"`
	FilePath string `mapstructure:"file_path"`
}

var GlobalConfig *Config

// InitConfig 初始化配置
func InitConfig(configPath string) error {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置到结构体
	GlobalConfig = &Config{}
	if err := viper.Unmarshal(GlobalConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	log.Printf("配置文件加载成功: %s", configPath)
	return nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 数据库默认配置
	viper.SetDefault("database.type", "mysql")
	viper.SetDefault("database.mysql.host", "localhost")
	viper.SetDefault("database.mysql.port", 3306)
	viper.SetDefault("database.mysql.charset", "utf8mb4")
	viper.SetDefault("database.mysql.parseTime", true)
	viper.SetDefault("database.mysql.loc", "Local")
	viper.SetDefault("database.sqlite.path", "./data/patreon.db")

	// 服务器默认配置
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")

	// 评测服务默认配置
	viper.SetDefault("judge.mode", "local")
	viper.SetDefault("judge.timeout", 15)
	viper.SetDefault("judge.queue_size", 100)
	viper.SetDefault("judge.local.enabled", true)
	viper.SetDefault("judge.local.sandbox_dir", "./sandbox")
	viper.SetDefault("judge.local.max_memory", 128)
	viper.SetDefault("judge.local.max_time", 5)
	viper.SetDefault("judge.local.max_output_size", 1024)
	viper.SetDefault("judge.local.supported_languages", []string{"go", "python", "cpp", "java"})

	viper.SetDefault("judge.local.executor", "host")
	viper.SetDefault("judge.local.docker_image_go", "golang:1.22-bookworm")
	viper.SetDefault("judge.local.docker_image_cpp", "gcc:13-bookworm")
	viper.SetDefault("judge.local.docker_image_python", "python:3.12-bookworm")
	viper.SetDefault("judge.local.docker_image_java", "eclipse-temurin:21-jdk")

	// 日志默认配置
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output", "stdout")

	// 图数据库默认配置
	viper.SetDefault("graph_database.neo4j.uri", "bolt://localhost:7687")
	viper.SetDefault("graph_database.neo4j.username", "neo4j")
	viper.SetDefault("graph_database.neo4j.password", "password")
	viper.SetDefault("graph_database.neo4j.database", "neo4j")
}

// GetDatabaseDSN 根据配置类型获取数据库连接字符串
func (c *Config) GetDatabaseDSN() string {
	switch c.Database.Type {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
			c.Database.MySQL.Username,
			c.Database.MySQL.Password,
			c.Database.MySQL.Host,
			c.Database.MySQL.Port,
			c.Database.MySQL.DBName,
			c.Database.MySQL.Charset,
			c.Database.MySQL.ParseTime,
			c.Database.MySQL.Loc,
		)
	case "sqlite":
		return c.Database.SQLite.Path
	default:
		log.Fatalf("不支持的数据库类型: %s", c.Database.Type)
		return ""
	}
}

// GetServerAddr 获取服务器地址
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf(":%d", c.Server.Port)
}
