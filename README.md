# PatreonOJ - 智能在线判题系统

<p align="center">
  <strong>一个功能完善的在线编程评测系统，集成知识图谱与智能推荐</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License">
  <img src="https://img.shields.io/badge/Database-MySQL%20%7C%20SQLite-orange" alt="Database">
  <img src="https://img.shields.io/badge/Graph-Neo4j-008CC1?logo=neo4j" alt="Neo4j">
</p>

---

## 📖 目录

- [✨ 特性](#-特性)
- [🛠️ 技术栈](#️-技术栈)
- [📦 项目结构](#-项目结构)
- [🚀 快速开始](#-快速开始)
- [⚙️ 配置说明](#️-配置说明)
- [📚 API 文档](#-api-文档)

---

## ✨ 特性

| 模块 | 功能描述 |
|------|---------|
| **🧑‍💻 用户系统** | 注册登录、权限管理、个人信息、学习进度追踪 |
| **📝 题目管理** | 题目 CRUD、分类标签、难度分级、搜索筛选 |
| **⚡ 代码评测** | 支持 Go/C++/Python/Java，Docker 沙箱隔离，资源限制 |
| **🧪 测试用例** | 批量导入、OSS 存储、隐藏/公开测试用例 |
| **📊 知识图谱** | Neo4j 存储题目关系、前置知识、学习路径推荐 |
| **📈 智能推荐** | 结合用户能力模型与知识图谱，分析知识盲区，提供靶向强化题目 |
| **🧠 能力评估** | 基于做题记录自动计算技能掌握度，支持雷达图展示（六边形战士） |
| **☁️ OSS 存储** | MinIO 对象存储，支持前端直传、预签名 URL |

---

## 🛠️ 技术栈

```
后端框架   │ Gin (Go Web Framework)
ORM       │ GORM
关系数据库 │ MySQL / SQLite
图数据库   │ Neo4j
对象存储   │ MinIO
配置管理   │ Viper
容器化评测 │ Docker
```

---

## 📦 项目结构

```
PatreonOJ/
├── cmd/PatreonOJ/              # 程序入口
│   └── main.go
├── internal/                   # 内部模块
│   ├── Controllers/            # 控制器层
│   │   ├── admin/              #   └─ CRUD 控制器
│   │   ├── graph_controller.go #   └─ 知识图谱 API
│   │   └── osscontroller.go    #   └─ OSS 接口
│   ├── models/                 # 数据模型层
│   │   ├── core.go             #   └─ DB 初始化 & 迁移
│   │   ├── user.go             #   └─ 用户模型
│   │   ├── question.go         #   └─ 题目模型
│   │   └── submission.go       #   └─ 提交记录模型
│   ├── graph/                  # Neo4j 图数据库
│   │   ├── neo4j.go            #   └─ 连接管理
│   │   └── question_graph.go   #   └─ 图操作逻辑
│   ├── services/               # 业务逻辑层
│   │   ├── judge_service.go    #   └─ 评测调度
│   │   ├── local_judge.go      #   └─ 本地评测实现
│   │   ├── ai_service.go       #   └─ AI 服务 (LLM集成)
│   │   ├── assessment_service.go # └─ 能力评估服务
│   │   └── recommendation_service.go # └─ 推荐服务
│   ├── routers/                # 路由配置
│   ├── oss/                    # OSS 客户端
│   ├── config/                 # 配置结构
│   └── util/                   # 工具函数
├── sandbox/                    # 沙箱目录（评测）
├── data/                       # 数据目录（SQLite）
├── config.yaml                 # 配置文件
└── go.mod / go.sum             # Go 模块依赖
```

---

## 🚀 快速开始

### 1️⃣ 安装依赖

```bash
go mod tidy
```

### 2️⃣ 配置数据库

编辑 `config.yaml`：

<details>
<summary><b>SQLite（推荐开发环境）</b></summary>

```yaml
database:
  type: "sqlite"
  sqlite:
    path: "./data/patreon.db"
```
</details>

<details>
<summary><b>MySQL（生产环境）</b></summary>

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
```
</details>

### 3️⃣ 启动服务

```bash
# 开发模式
go run cmd/PatreonOJ/main.go

# 或编译后运行
go build -o PatreonOJ.exe cmd/PatreonOJ/main.go
./PatreonOJ.exe
```

服务默认运行在 `http://localhost:8080`

---

## ⚙️ 配置说明

### 评测系统配置

```yaml
### 评测系统配置

```yaml
judge:
  mode: "local"                     # local (本地Docker) / remote (外部API)
  timeout: 15                       # 评测超时时间(秒)
  queue_size: 100                   # 评测队列深度

  # Go-Judge 高效沙箱 (推荐)
  go_judge:
    enabled: true
    api_url: "http://localhost:5050/run"
    token: ""

  # 本地 Docker 评测 (备用)
  local:
    enabled: true
    executor: docker
    sandbox_dir: ./sandbox
    max_memory: 256                 # MB
    max_time: 5000                  # ms
    max_output_size: 1024           # KB
    docker_image_go: golang:1.22-bookworm
    docker_image_cpp: gcc:13-bookworm
    docker_image_python: python:3.12-bookworm
    docker_image_java: eclipse-temurin:21-jdk
```

### Neo4j 图数据库（可选）

```yaml
graph_database:
  neo4j:
    uri: "bolt://localhost:7687"
    username: "neo4j"
    password: "password"
    database: "neo4j"
```

### 日志配置

```yaml
log:
  level: "info"       # debug, info, warn, error
  format: "json"      # json, text
  output: "stdout"    # stdout, file
  file_path: "./logs/app.log"
```

### MinIO OSS 存储

```yaml
oss:
  address: "localhost:9090"
  public_address: "localhost:9090"
  access_key: "your_access_key"
  secret_key: "your_secret_key"
  bucket_name: "patreon-oj-cases"
  public_read_prefixes: ["avatars/"]
```

### AI 服务配置（可选）

支持 OpenAI 及兼容 API（如 Ollama 本地部署）：

<details>
<summary><b>Ollama 本地部署（推荐）</b></summary>

```yaml
ai:
  enabled: true
  base_url: "http://localhost:11434/v1"
  api_key: ""  # Ollama 不需要 API Key
  model: "deepseek-r1:8b"  # 或 qwen2.5:7b, llama3:8b
  temperature: 0.7
```
</details>

<details>
<summary><b>OpenAI / 云端 API</b></summary>

```yaml
ai:
  enabled: true
  base_url: "https://api.openai.com/v1"
  api_key: "sk-your-api-key"
  model: "gpt-3.5-turbo"
  temperature: 0.7
```
</details>

---

## 📚 API 文档

### 用户管理 `/user`

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/user/` | 获取用户列表 |
| POST | `/user/register` | 用户注册 |
| POST | `/user/login` | 用户登录 |
| POST | `/user/logout` | 用户注销 |
| GET | `/user/:uuid` | 获取用户信息 |
| PUT | `/user/:uuid` | 更新用户信息 |
| GET | `/user/solves/:uuid` | 获取用户解题ID列表 |
| GET | `/user/solve/` | 查询某题是否已解决 (`?question_number=`) |
| GET | `/user/:uuid/mastery/questions` | 查询题目掌握度 |
| GET | `/user/:uuid/mastery/tags` | 查询标签掌握度 |
| POST | `/user/:uuid/mastery/events` | 提交学习事件 |
| GET | `/api/v1/user/stats/radar` | 获取用户能力雷达图数据 |

<details>
<summary><b>请求/响应示例</b></summary>

**注册** `POST /user/register`
```json
{
    "username": "用户名",
    "password": "密码"
}
```

**登录** `POST /user/login`
```json
{
    "username": "用户名",
    "password": "密码"
}
```

**能力雷达图** `GET /api/v1/user/stats/radar`
```json
{
  "code": 200,
  "data": [
    { "subject": "Array", "A": 85, "fullMark": 100 },
    { "subject": "DP", "A": 60, "fullMark": 100 },
    { "subject": "Greedy", "A": 40, "fullMark": 100 }
  ]
}
```

**技能掌握度** `GET /user/:uuid/mastery/tags`
```json
{
  "code": 200,
  "data": [
    { "skill_key": "array", "mastery": 0.85 },
    { "skill_key": "dynamic_programming", "mastery": 0.60 }
  ]
}
```

**题目掌握情况** `GET /user/:uuid/mastery/questions`
```json
{
  "code": 200,
  "data": [
    { "question_number": 1001, "mastery": 1.0, "last_updated": "2024-03-20T10:00:00Z" },
    { "question_number": 1005, "mastery": 0.5, "last_updated": "2024-03-21T15:30:00Z" }
  ]
}
```
</details>

---

### 题目管理 `/question`

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/question/` | 获取题目列表（支持 `?q=` 搜索） |
| GET | `/question/:number` | 按题号获取题目 |
| GET | `/question/new` | 获取最新题目 |
| POST | `/question/` | 创建题目 |
| POST | `/question/:number` | 更新题目 |

<details>
<summary><b>请求/响应示例</b></summary>

**创建题目** `POST /question/`
```json
{
    "question_id": "p1001",
    "title": "两数之和",
    "content": "题目描述...",
    "difficulty": "简单",
    "time_limit": 1000,
    "memory_limit": 128,
    "tags": "数组,哈希表",
    "status": "published"
}
```
</details>

---

### 代码评测 `/submission`

| 方法 | 路径 | 描述 |
|-----|------|------|
| POST | `/submission/` | 提交代码 |
| GET | `/submission/:id` | 获取评测结果 |
| GET | `/api/problems/:number/submissions` | 题目提交记录（公开） |
| GET | `/api/users/:user_id/submissions` | 个人提交记录 |

<details>
<summary><b>请求/响应示例</b></summary>

**提交代码** `POST /submission/`
```json
{
    "user_id": "用户UUID",
    "question_number": 1001,
    "code": "package main...",
    "language": "go"
}
```

**支持语言**: `go`, `cpp`, `python`, `java`

**评测状态**:
- `pending` - 等待评测
- `judging` - 评测中
- `accepted` - 通过
- `wrong_answer` - 答案错误
- `time_limit_exceeded` - 超时
- `memory_limit_exceeded` - 内存超限
- `runtime_error` - 运行时错误
- `compile_error` - 编译错误
</details>

---

### 测试用例 `/testcase`

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/testcase/` | 获取测试用例列表 |
| GET | `/testcase/question/:number` | 按题号获取测试用例 |
| POST | `/testcase/` | 添加单个测试用例 |
| POST | `/testcase/batch` | 批量添加测试用例 |
| POST | `/testcase/oss/commit` | OSS 上传后落库 |
| PUT | `/testcase/:id` | 更新测试用例 |
| DELETE | `/testcase/:id` | 删除测试用例 |

---

### 知识图谱 `/graph`

> ⚠️ 以下接口需要 Neo4j 连接成功才可用

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/graph/node` | 获取全部节点和边（用于前端可视化） |
| POST | `/graph/questions/:number/sync` | 同步题目到 Neo4j |
| POST | `/graph/relations` | 创建题目关系边 |
| DELETE | `/graph/relations` | 删除题目关系边 |
| GET | `/graph/questions/:number/prerequisites` | 查询前置题 |
| GET | `/graph/questions/:number/next` | 查询进阶题 |
| GET | `/graph/questions/:number/recommendations` | 获取推荐题目 |
| GET | `/graph/path?start=&end=` | 查找学习路径 |

#### AI 智能分析（需启用 AI 配置）

| 方法 | 路径 | 描述 |
|-----|------|------|
| POST | `/graph/analyze/questions/:number` | AI 分析题目关系（前置/相似） |
| POST | `/graph/analyze/skills` | AI 自动构建技能树 |

<details>
<summary><b>请求/响应示例</b></summary>

**创建关系边** `POST /graph/relations`
```json
{
  "from_question": 1001,
  "to_question": 1002,
  "relation_type": "PREREQUISITE",
  "weight": 0.9,
  "description": "1001 是 1002 的前置基础"
}
```

**AI 分析技能树** `POST /graph/analyze/skills`
```json
// 响应示例
{
  "message": "技能树分析完成",
  "relations": [
    {"parent_skill": "数组", "child_skill": "双指针", "reason": "双指针常用于处理数组结构中的问题"},
    {"parent_skill": "回溯", "child_skill": "组合枚举", "reason": "回溯算法用于生成所有可能的组合"}
  ],
  "saved": 2,
  "failed": 0
}
```

**推荐题目响应** `GET /graph/questions/1001/recommendations`
```json
{
  "question_number": 1001,
  "recommendations": [
    {
      "question_number": 1002,
      "title": "三数之和",
      "difficulty": "中等",
      "score": 0.82,
      "reason": "进阶题目",
      "relation_type": "NEXT_LEVEL"
    }
  ]
}
```
</details>

---

### 智能推荐 `/api/v1/recommendations`

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/recommendations` | 获取个性化推荐题目 |

**参数说明**:
- `limit`: 返回数量 (默认 10)

```json
// 响应示例
{
  "code": 200,
  "data": [
    {
      "question_number": 1002,
      "title": "三数之和",
      "difficulty": "中等",
      "reason": "针对性强化: 数组 (当前: 0.45)",
      "score": 1.0
    }
  ]
}
```

---

### OSS 文件管理 `/oss`

| 方法 | 路径 | 描述 |
|-----|------|------|
| POST | `/oss/upload` | 上传文件 |
| GET | `/oss/upload-url` | 获取预签名上传 URL |
| GET | `/oss/files` | 列出目录内容 |

**前端直传流程**:
1. `GET /oss/upload-url?filename=input.txt&path=problems/1001/` → 获取预签名 URL
2. 前端 PUT 文件到预签名 URL
3. `POST /testcase/oss/commit` → 落库

---

### 其他接口

| 模块 | 路径 | 描述 |
|-----|------|------|
| 分类 | `/category/` | 分类 CRUD |
| 节点 | `/node/` | 获取节点列表 |
| 关系 | `/relation/` | 获取关系列表 |
| 首页 | `/overview/getHomeText` | 获取首页文本 |
| 公告 | `/overview/getAnnouncement` | 获取公告 |

---

## 📄 License

MIT License © 2024-2026

---

<p align="center">
  <sub>Built with ❤️ using Go</sub>
</p>

---