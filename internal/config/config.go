package config

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

// Config 全局配置结构体
type Config struct {
	Database      DatabaseConfig      `mapstructure:"database"`
	GraphDatabase GraphDatabaseConfig `mapstructure:"graph_database"`
	Server        ServerConfig        `mapstructure:"server"`
	Judge         JudgeConfig         `mapstructure:"judge"`
	OSS           OSSConfig           `mapstructure:"oss"`
	AI            AIConfig            `mapstructure:"ai"`
}

// AIConfig AI服务配置
type AIConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	APIKey      string  `mapstructure:"api_key"`
	BaseURL     string  `mapstructure:"base_url"`
	Model       string  `mapstructure:"model"`
	Temperature float64 `mapstructure:"temperature"`
}

// OSSConfig OSS配置
type OSSConfig struct {
	Address            string   `mapstructure:"address"`
	PublicAddress      string   `mapstructure:"public_address"`
	PublicPathPrefix   string   `mapstructure:"public_path_prefix"`
	PublicReadPrefixes []string `mapstructure:"public_read_prefixes"`
	AccessKey          string   `mapstructure:"access_key"`
	SecretKey          string   `mapstructure:"secret_key"`
	BucketName         string   `mapstructure:"bucket_name"`
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
	Enabled bool        `mapstructure:"enabled"`
	Neo4j   Neo4jConfig `mapstructure:"neo4j"`
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
	Host                  string   `mapstructure:"host"`
	Port                  int      `mapstructure:"port"`
	Mode                  string   `mapstructure:"mode"`
	SecureCookies         bool     `mapstructure:"secure_cookies"`
	TrustedProxies        []string `mapstructure:"trusted_proxies"`
	AuthMaxConcurrent     int      `mapstructure:"auth_max_concurrent"`
	AuthRequestsPerMinute int      `mapstructure:"auth_requests_per_minute"`
}

// JudgeConfig 评测服务配置
type JudgeConfig struct {
	Mode      string           `mapstructure:"mode"`
	Timeout   int              `mapstructure:"timeout"`
	QueueSize int              `mapstructure:"queue_size"`
	Local     LocalJudgeConfig `mapstructure:"local"`
	GoJudge   GoJudgeConfig    `mapstructure:"go_judge"`
}

type GoJudgeConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	APIURL    string `mapstructure:"api_url"`
	Token     string `mapstructure:"token"`
	MaxMemory int    `mapstructure:"max_memory"`
	MaxTime   int    `mapstructure:"max_time"`
}

// LocalJudgeConfig 本地评测配置
type LocalJudgeConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	SandboxDir         string   `mapstructure:"sandbox_dir"`
	MaxMemory          int      `mapstructure:"max_memory"`
	MaxTime            int      `mapstructure:"max_time"`
	MaxOutputSize      int      `mapstructure:"max_output_size"`
	MaxWorkSize        int      `mapstructure:"max_work_size"` // MiB, bounded tmpfs
	SupportedLanguages []string `mapstructure:"supported_languages"`

	Executor string `mapstructure:"executor"` // host/docker

	DockerImageGo     string `mapstructure:"docker_image_go"`
	DockerImageCpp    string `mapstructure:"docker_image_cpp"`
	DockerImagePython string `mapstructure:"docker_image_python"`
	DockerImageJava   string `mapstructure:"docker_image_java"`
}

var GlobalConfig *Config

// InitConfig 初始化配置
func InitConfig(configPath string) error {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 设置默认值
	setDefaults(v)
	v.SetEnvPrefix("PATREONOJ")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置到结构体
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	GlobalConfig = cfg

	log.Printf("配置文件加载成功: %s", configPath)
	return nil
}

// setDefaults 设置默认配置值
func setDefaults(v *viper.Viper) {
	// 数据库默认配置
	v.SetDefault("database.type", "mysql")
	v.SetDefault("database.mysql.host", "localhost")
	v.SetDefault("database.mysql.port", 3306)
	v.SetDefault("database.mysql.charset", "utf8mb4")
	v.SetDefault("database.mysql.parseTime", true)
	v.SetDefault("database.mysql.loc", "Local")
	v.SetDefault("database.sqlite.path", "./data/patreon.db")

	// 服务器默认配置
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.secure_cookies", false)
	v.SetDefault("server.trusted_proxies", []string{})
	v.SetDefault("server.auth_max_concurrent", 2)
	v.SetDefault("server.auth_requests_per_minute", 20)
	v.SetDefault("server.mode", "debug")

	// 评测服务默认配置
	v.SetDefault("judge.mode", "local")
	v.SetDefault("judge.timeout", 15)
	v.SetDefault("judge.queue_size", 100)
	v.SetDefault("judge.local.enabled", true)
	v.SetDefault("judge.local.sandbox_dir", "./sandbox")
	v.SetDefault("judge.local.max_memory", 128)
	v.SetDefault("judge.local.max_time", 5000)
	v.SetDefault("judge.local.max_output_size", 1024)
	v.SetDefault("judge.local.max_work_size", 256)
	v.SetDefault("judge.local.supported_languages", []string{"go", "python", "cpp", "java"})

	v.SetDefault("judge.go_judge.max_memory", 256)
	v.SetDefault("judge.go_judge.max_time", 5000)
	v.SetDefault("judge.local.executor", "docker")
	v.SetDefault("judge.local.docker_image_go", "golang:1.22-bookworm")
	v.SetDefault("judge.local.docker_image_cpp", "gcc:13-bookworm")
	v.SetDefault("judge.local.docker_image_python", "python:3.12-bookworm")
	v.SetDefault("judge.local.docker_image_java", "eclipse-temurin:21-jdk")

	// Oss默认公开读取前缀
	v.SetDefault("oss.public_read_prefixes", []string{})

	// 图数据库默认配置
	v.SetDefault("graph_database.enabled", false)
	v.SetDefault("graph_database.neo4j.uri", "bolt://localhost:7687")
	v.SetDefault("graph_database.neo4j.username", "neo4j")
	v.SetDefault("graph_database.neo4j.password", "password")
	v.SetDefault("graph_database.neo4j.database", "neo4j")

	// AI服务默认配置
	v.SetDefault("ai.enabled", false)
	v.SetDefault("ai.base_url", "https://api.openai.com/v1") // 默认OpenAI地址
	v.SetDefault("ai.model", "gpt-3.5-turbo")
	v.SetDefault("ai.temperature", 0.7)
}

// GetDatabaseDSN 根据配置类型获取数据库连接字符串
func (c *Config) GetDatabaseDSN() string {
	switch c.Database.Type {
	case "mysql":
		cfg := mysqldriver.NewConfig()
		cfg.User, cfg.Passwd = c.Database.MySQL.Username, c.Database.MySQL.Password
		cfg.Net, cfg.Addr = "tcp", net.JoinHostPort(c.Database.MySQL.Host, strconv.Itoa(c.Database.MySQL.Port))
		cfg.DBName, cfg.ParseTime = c.Database.MySQL.DBName, c.Database.MySQL.ParseTime
		cfg.Loc, _ = time.LoadLocation(c.Database.MySQL.Loc)
		if cfg.Loc == nil {
			cfg.Loc = time.Local
		}
		cfg.Params = map[string]string{"charset": c.Database.MySQL.Charset}
		cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
		return cfg.FormatDSN()
	case "sqlite":
		return c.Database.SQLite.Path
	default:
		return ""
	}
}

func (c *Config) Validate() error {
	c.Database.Type = strings.ToLower(strings.TrimSpace(c.Database.Type))
	if c.Database.Type != "mysql" && c.Database.Type != "sqlite" {
		return fmt.Errorf("不支持的数据库类型: %s", c.Database.Type)
	}
	if c.Database.Type == "mysql" {
		if _, err := time.LoadLocation(c.Database.MySQL.Loc); err != nil {
			return fmt.Errorf("数据库时区配置错误: %w", err)
		}
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if c.Server.AuthMaxConcurrent < 1 || c.Server.AuthMaxConcurrent > 8 || c.Server.AuthRequestsPerMinute < 1 {
		return fmt.Errorf("server.auth_max_concurrent must be between 1 and 8; auth_requests_per_minute must be positive")
	}
	if c.Judge.Local.MaxWorkSize < 16 || c.Judge.Local.MaxWorkSize > 1024 {
		return fmt.Errorf("judge.local.max_work_size must be between 16 and 1024 MiB")
	}
	if c.Server.Mode != "debug" && c.Server.Mode != "release" && c.Server.Mode != "test" {
		return fmt.Errorf("server.mode must be debug, release or test")
	}
	c.Judge.Mode = strings.ToLower(strings.TrimSpace(c.Judge.Mode))
	if c.Judge.Mode != "local" && c.Judge.Mode != "remote" {
		return fmt.Errorf("judge.mode must be local or remote")
	}
	if c.Judge.Timeout <= 0 || c.Judge.QueueSize <= 0 {
		return fmt.Errorf("judge.timeout and judge.queue_size must be positive")
	}
	if c.Judge.Local.MaxTime <= 0 || c.Judge.Local.MaxMemory <= 0 || c.Judge.Local.MaxOutputSize <= 0 || c.Judge.GoJudge.MaxTime <= 0 || c.Judge.GoJudge.MaxMemory <= 0 {
		return fmt.Errorf("judge limits must be positive")
	}
	if c.Judge.Mode == "local" && (!c.Judge.Local.Enabled || strings.ToLower(strings.TrimSpace(c.Judge.Local.Executor)) != "docker") {
		return fmt.Errorf("local judge requires enabled=true and executor=docker")
	}
	if c.Judge.Mode == "remote" && (!c.Judge.GoJudge.Enabled || strings.TrimSpace(c.Judge.GoJudge.APIURL) == "") {
		return fmt.Errorf("remote judge requires go_judge.enabled=true and a valid api_url")
	}
	return nil
}

// GetServerAddr 获取服务器地址
func (c *Config) GetServerAddr() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}
