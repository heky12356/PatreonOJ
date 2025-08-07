# PatreonOJ - 在线判题系统

## 项目简介

PatreonOJ 是一个基于 Go + Gin + GORM 的在线判题系统，支持多种数据库类型，具有完整的用户管理、题目管理和代码评测功能。

## 新特性

### 🎯 多数据库支持
- **MySQL**: 适用于生产环境的高性能数据库
- **SQLite**: 适用于开发和测试环境的轻量级数据库

### ⚙️ 配置管理
- 使用 Viper 进行配置管理
- 支持 YAML 格式配置文件
- 支持环境变量和默认值

### 🔧 灵活配置
- 数据库类型可配置切换
- 服务器端口和运行模式可配置
- 评测服务参数可配置
- 日志级别和格式可配置

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置数据库

编辑 `config/config.yaml` 文件：

#### 使用 MySQL
```yaml
database:
  type: "mysql"
  mysql:
    host: "localhost"
    port: 3306
    username: "root"
    password: "your_password"
    dbname: "patreon_oj"
    charset: "utf8mb4"
    parseTime: true
    loc: "Local"
```

#### 使用 SQLite
```yaml
database:
  type: "sqlite"
  sqlite:
    path: "./data/patreon.db"
```

### 3. 启动服务

```bash
go run main.go
```

服务器将在配置的端口启动（默认 8080）。

## 配置说明

### 数据库配置
- `database.type`: 数据库类型，支持 "mysql" 或 "sqlite"
- `database.mysql.*`: MySQL 相关配置
- `database.sqlite.path`: SQLite 数据库文件路径

### 服务器配置
- `server.port`: 服务器端口（默认 8080）
- `server.mode`: Gin 运行模式（debug/release/test）

### 评测服务配置
- `judge.api_url`: 评测服务 API 地址
- `judge.timeout`: 评测超时时间（秒）
- `judge.queue_size`: 评测队列大小

### 日志配置
- `log.level`: 日志级别（debug/info/warn/error）
- `log.format`: 日志格式（json/text）
- `log.output`: 日志输出（stdout/file）

## API 接口

### 用户管理
- `POST /user/register` - 用户注册
- `POST /user/login` - 用户登录
- `POST /user/logout` - 用户注销
- `GET /user/` - 获取用户列表

### 题目管理
- `GET /question/` - 获取题目列表
- `POST /question/` - 创建题目
- `POST /question/:id` - 更新题目

### 分类管理
- `GET /category/` - 获取分类列表
- `POST /category/` - 创建分类
- `POST /category/:id` - 更新分类

### 关系管理
- `GET /relation/` - 获取关系列表

### 节点管理
- `GET /node/` - 获取节点列表

### 代码评测
- `POST /submission/` - 提交代码
- `GET /submission/:id` - 获取提交结果

## 项目结构

```
PatreonOJ/
├── config/                 # 配置文件和配置管理
│   ├── config.yaml        # 主配置文件
│   └── config.go          # 配置结构体和初始化
├── Controllers/           # 控制器层
│   └── admin/            # 管理员控制器
├── models/               # 数据模型层
│   ├── core.go          # 数据库初始化
│   └── *.go             # 各种数据模型
├── routers/             # 路由层
├── services/            # 服务层
└── main.go             # 程序入口
```

## 开发说明

### 数据库迁移
系统启动时会自动执行数据库表迁移，无需手动创建表结构。

### 添加新模型
1. 在 `models/` 目录下创建新的模型文件
2. 在 `models/core.go` 的 `AutoMigrate()` 函数中添加新模型

### 环境切换
通过修改 `config/config.yaml` 中的 `database.type` 字段即可切换数据库类型：
- 开发环境：推荐使用 SQLite
- 生产环境：推荐使用 MySQL

## 注意事项

1. **SQLite 数据库文件**: 确保 SQLite 数据库文件的目录具有写权限
2. **MySQL 连接**: 确保 MySQL 服务正在运行且连接信息正确
3. **配置文件**: 首次运行前请检查并修改配置文件中的相关参数
4. **端口占用**: 确保配置的端口未被其他程序占用

## 技术栈

- **后端框架**: Gin
- **ORM**: GORM
- **数据库**: MySQL / SQLite
- **配置管理**: Viper
- **UUID**: Google UUID
- **模板引擎**: Go HTML Template