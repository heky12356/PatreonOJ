# PatreonOJ - 在线判题系统

## 项目简介

PatreonOJ 是一个基于 Go + Gin + GORM 的在线判题系统，支持多种数据库类型和评测模式，具有完整的用户管理、题目管理和代码评测功能。

## 核心特性

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

### 📝 测试用例管理
- 完整的测试用例CRUD操作
- 支持单个和批量添加测试用例
- 支持按题目筛选查询测试用例
- 支持隐藏测试用例功能
- 与评测系统完全集成

### 🏃‍♂️ 双模式评测系统
- **本地评测**: 支持Go、Python、C++、Java等多种语言的本地编译执行
- **远程评测**: 支持调用外部评测API服务
- **沙箱隔离**: 本地评测采用沙箱机制确保安全性
- **实时监控**: 支持内存、时间、输出大小限制

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置数据库

编辑 `config.yaml` 文件：

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

### 3. 配置评测系统

#### 本地评测模式（推荐）
```yaml
judge:
  mode: "local"  # 使用本地评测
  local:
    enabled: true
    sandbox_dir: "./sandbox"  # 沙箱目录
    max_memory: 128  # 最大内存限制(MB)
    max_time: 5  # 最大执行时间(秒)
    max_output_size: 1024  # 最大输出大小(KB)
    supported_languages:
      - "go"
      - "python"
      - "cpp"
      - "java"
```

#### 远程评测模式
```yaml
judge:
  mode: "remote"  # 使用远程API评测
  api_url: "http://your-judge-service-api"
  timeout: 15  # 超时时间（秒）
  queue_size: 100  # 队列大小
```

### 4. 启动服务

```bash
go run cmd/PatreonOJ/main.go
```

或者编译后运行：

```bash
go build -o PatreonOJ.exe cmd/PatreonOJ/main.go
./PatreonOJ.exe
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
- `judge.mode`: 评测模式，支持 "local"（本地评测）或 "remote"（远程API评测）
- `judge.api_url`: 远程评测服务 API 地址（仅远程模式需要）
- `judge.timeout`: 评测超时时间（秒）
- `judge.queue_size`: 评测队列大小

#### 本地评测配置
- `judge.local.enabled`: 是否启用本地评测
- `judge.local.sandbox_dir`: 沙箱目录路径
- `judge.local.max_memory`: 最大内存限制（MB）
- `judge.local.max_time`: 最大执行时间（秒）
- `judge.local.max_output_size`: 最大输出大小（KB）
- `judge.local.supported_languages`: 支持的编程语言列表

### 日志配置
- `log.level`: 日志级别（debug/info/warn/error）
- `log.format`: 日志格式（json/text）
- `log.output`: 日志输出（stdout/file）
- `log.file_path`: 日志文件路径（仅文件输出模式需要）

## API 接口

### 用户管理
- `GET /user/` - 获取用户列表
- `POST /user/register` - 用户注册
- `POST /user/login` - 用户登录
- `POST /user/logout` - 用户注销

#### POST 接口 JSON 格式

**用户注册** `POST /user/register`
```json
{
    "username": "用户名",
    "password": "密码"
}
```

**用户登录** `POST /user/login`
```json
{
    "username": "用户名",
    "password": "密码"
}
```

**用户注销** `POST /user/logout`
```json
{
    "user_id": 1
}
```

### 题目管理
- `GET /question/` - 获取题目列表（按题目编号排序）
- `GET /question/:number` - 通过题目编号获取单个题目详情
- `POST /question/` - 创建题目
- `POST /question/:number` - 更新题目（使用题目编号）

#### POST 接口 JSON 格式

**创建题目** `POST /question/`

完整版示例（包含所有可选字段）：
```json
{
    "question_number": 1001,
    "title": "两数之和",
    "content": "给定一个整数数组 nums 和一个整数目标值 target，请你在该数组中找出和为目标值 target 的那两个整数，并返回它们的数组下标。",
    "difficulty": "简单",
    "input_format": "第一行包含一个整数 n，表示数组长度。\n第二行包含 n 个整数，表示数组元素。\n第三行包含一个整数 target，表示目标值。",
    "output_format": "输出两个整数，表示两个数的下标（从0开始）。",
    "sample_input": "4\n2 7 11 15\n9",
    "sample_output": "0 1",
    "sample_explanation": "因为 nums[0] + nums[1] = 2 + 7 = 9，所以返回 [0, 1]。",
    "data_range": "2 ≤ n ≤ 10^4\n-10^9 ≤ nums[i] ≤ 10^9\n-10^9 ≤ target ≤ 10^9",
    "time_limit": 1000,
    "memory_limit": 128,
    "source": "LeetCode",
    "tags": "数组,哈希表",
    "hint": "可以使用哈希表来优化查找过程",
    "category_id": 1,
    "status": "published"
}
```

最简版示例（仅必填字段）：
```json
{
    "title": "两数之和",
    "content": "给定一个整数数组 nums 和一个整数目标值 target，请你在该数组中找出和为目标值 target 的那两个整数，并返回它们的数组下标。",
    "difficulty": "简单"
}
```

**字段说明：**
- `question_number`: 题目编号（可选，不提供时自动从1001开始递增）
- `title`: 题目标题（必填）
- `content`: 题目描述（必填）
- `difficulty`: 难度等级（必填）
- `input_format`: 输入格式说明（可选）
- `output_format`: 输出格式说明（可选）
- `sample_input`: 样例输入（可选）
- `sample_output`: 样例输出（可选）
- `sample_explanation`: 样例解释（可选）
- `data_range`: 数据范围约束（可选）
- `time_limit`: 时间限制，单位毫秒（可选，默认2000）
- `memory_limit`: 内存限制，单位MB（可选，默认256）
- `source`: 题目来源（可选）
- `tags`: 题目标签，逗号分隔（可选）
- `hint`: 提示信息（可选）
- `status`: 题目状态（可选，默认"draft"，可选值：draft/published/archived）
- `category_id`: 分类ID（可选）

**更新题目** `POST /question/:number`
```json
{
    "title": "更新后的题目标题",
    "content": "更新后的题目内容",
    "difficulty": "更新后的难度",
    "input_format": "更新后的输入格式",
    "output_format": "更新后的输出格式",
    "time_limit": 3000,
    "memory_limit": 512,
    "status": "published",
    "category_id": 2
}
```

### 分类管理
- `GET /category/` - 获取分类列表
- `POST /category/` - 创建分类
- `POST /category/:id` - 更新分类

#### POST 接口 JSON 格式

**创建分类** `POST /category/`
```json
{
    "name": "分类名称"
}
```

**更新分类** `POST /category/:id`
```json
{
    "name": "更新后的分类名称"
}
```

### 关系管理
- `GET /relation/` - 获取关系列表

### 节点管理
- `GET /node/` - 获取节点列表

### 测试用例管理
- `GET /testcase/` - 获取测试用例列表
- `GET /testcase/question/:number` - 根据题目编号获取测试用例
- `GET /testcase/:id` - 获取单个测试用例详情
- `POST /testcase/` - 添加单个测试用例
- `POST /testcase/batch` - 批量添加测试用例
- `PUT /testcase/:id` - 更新测试用例
- `DELETE /testcase/:id` - 删除测试用例

#### POST 接口 JSON 格式

**添加单个测试用例** `POST /testcase/`
```json
{
    "question_number": 1001,
    "input": "1 2",
    "expected_output": "3",
    "is_hidden": false
}
```

**批量添加测试用例** `POST /testcase/batch`
```json
{
    "question_number": 1001,
    "test_cases": [
        {
            "input": "1 2",
            "expected_output": "3",
            "is_hidden": false
        },
        {
            "input": "5 10",
            "expected_output": "15",
            "is_hidden": true
        }
    ]
}
```

**字段说明：**
- `question_number`: 题目编号（必填）
- `input`: 输入数据（必填）
- `expected_output`: 期望输出（必填）
- `is_hidden`: 是否隐藏测试用例（可选，默认false）

### 代码评测
- `POST /submission/` - 提交代码
- `GET /submission/:id` - 获取提交结果

#### POST 接口 JSON 格式

**提交代码** `POST /submission/`
```json
{
    "user_id": "用户UUID",
    "question_number": 1001,
    "code": "提交的代码内容",
    "language": "cpp"
}
```

**字段说明：**
- `user_id`: 用户UUID（必填）
- `question_number`: 题目编号，如1001、1002等（必填）
- `code`: 提交的代码内容（必填）
- `language`: 编程语言（必填），支持的语言：
  - `go`: Go语言
  - `cpp`: C++语言
  - `python`: Python语言
  - `java`: Java语言

**响应示例：**
```json
{
    "submission_id": "550e8400-e29b-41d4-a716-446655440000",
    "user_id": "用户UUID",
    "question_number": 1001,
    "question_id": 1,
    "status": "pending",
    "message": "代码已提交，正在评测中",
    "created_at": "2024-01-01T12:00:00Z"
}
```

**评测结果状态：**
- `pending`: 等待评测
- `judging`: 评测中
- `accepted`: 通过
- `wrong_answer`: 答案错误
- `time_limit_exceeded`: 超时
- `memory_limit_exceeded`: 内存超限
- `runtime_error`: 运行时错误
- `compile_error`: 编译错误
- `system_error`: 系统错误


## 项目结构

```
PatreonOJ/
├── cmd/                   # 命令行工具
│   └── PatreonOJ/
│       └── main.go       # 程序入口
├── internal/             # 内部模块
│   ├── Controllers/      # 控制器层
│   │   └── admin/       # 管理员控制器
│   │       ├── userController.go      # 用户管理控制器
│   │       ├── questionController.go  # 题目管理控制器
│   │       ├── categoryController.go  # 分类管理控制器
│   │       ├── testCaseController.go  # 测试用例管理控制器
│   │       ├── nodeController.go      # 节点管理控制器
│   │       ├── relationController.go  # 关系管理控制器
│   │       └── submissionController.go # 提交管理控制器
│   ├── models/          # 数据模型层
│   │   ├── core.go     # 数据库初始化
│   │   ├── user.go     # 用户模型
│   │   ├── question.go # 题目和测试用例模型
│   │   ├── submission.go # 提交记录模型
│   │   ├── category.go # 分类模型
│   │   └── *.go        # 其他数据模型
│   ├── routers/        # 路由层
│   │   └── routers.go  # 路由配置
│   ├── services/       # 服务层
│   │   ├── judge_service.go # 评测服务
│   │   └── local_judge.go   # 本地评测服务
│   └── config/         # 配置管理
│       └── config.go   # 配置结构体
├── sandbox/            # 沙箱目录（本地评测）
├── data/              # 数据目录（SQLite数据库）
├── logs/              # 日志目录
├── config.yaml        # 配置文件
├── go.mod            # Go模块文件
├── go.sum            # Go模块依赖
└── README.md         # 项目说明文档
```


## 开发说明

### 数据库迁移
系统启动时会自动执行数据库表迁移，无需手动创建表结构。

### 添加新模型
1. 在 `internal/models/` 目录下创建新的模型文件
2. 在 `internal/models/core.go` 的 `AutoMigrate()` 函数中添加新模型

### 添加新控制器
1. 在 `internal/Controllers/admin/` 目录下创建新的控制器文件
2. 在 `internal/routers/routers.go` 中添加相应的路由配置

### 本地评测系统
本地评测系统提供完整的代码编译和执行功能：

#### 支持的编程语言
- **Go**: 使用 `go build` 编译，支持标准输入输出
- **C++**: 使用 `g++` 编译，支持标准输入输出
- **Python**: 直接使用 `python` 解释器执行
- **Java**: 使用 `javac` 编译，`java` 执行

#### 安全机制
- **沙箱隔离**: 每次评测在独立的沙箱目录中进行
- **资源限制**: 支持内存、时间、输出大小限制
- **自动清理**: 评测完成后自动清理临时文件

#### 环境要求
确保系统已安装相应的编译器和解释器：
- Go: `go version` 检查Go环境
- C++: `g++ --version` 检查GCC环境
- Python: `python --version` 检查Python环境
- Java: `javac -version` 和 `java -version` 检查Java环境


### 环境切换
通过修改 `config.yaml` 中的相关配置即可切换：

#### 数据库切换
修改 `database.type` 字段：
- 开发环境：推荐使用 SQLite
- 生产环境：推荐使用 MySQL

#### 评测模式切换
修改 `judge.mode` 字段：
- 本地评测：设置为 "local"（推荐）
- 远程评测：设置为 "remote"

## 注意事项

1. **SQLite 数据库文件**: 确保 SQLite 数据库文件的目录具有写权限
2. **MySQL 连接**: 确保 MySQL 服务正在运行且连接信息正确
3. **配置文件**: 首次运行前请检查并修改配置文件中的相关参数
4. **端口占用**: 确保配置的端口未被其他程序占用
5. **测试用例管理**: 每个题目至少需要一个测试用例才能进行代码评测
6. **数据完整性**: 删除题目时会自动删除相关的测试用例和提交记录
7. **本地评测环境**: 
   - 确保系统已安装所需的编译器和解释器
   - Windows系统需要正确配置PATH环境变量
   - 沙箱目录需要有读写权限
8. **路径问题**: 
   - Windows系统下注意路径分隔符问题
   - 建议使用绝对路径配置沙箱目录
9. **资源限制**: 
   - 本地评测的资源限制依赖于系统配置
   - 建议根据服务器性能调整相关参数

## 技术栈

- **后端框架**: Gin
- **ORM**: GORM
- **数据库**: MySQL / SQLite
- **配置管理**: Viper
- **UUID**: Google UUID
- **本地评测**: 
  - Go编译器
  - GCC (C++)
  - Python解释器
  - Java编译器和虚拟机
