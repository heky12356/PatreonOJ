# PatreonOJ - 在线判题系统

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

#### 本地评测模式
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
暂不支持

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
暂不支持

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

### 用户态数据
- `GET /user/:uuid?operator_uuid=...` - 获取用户信息
- `PUT /user/:uuid?operator_uuid=...` - 更新用户信息
- `GET /user/:uuid/mastery/questions?operator_uuid=...` - 查询题目掌握度（分页/筛选/排序）
- `GET /user/:uuid/mastery/tags?operator_uuid=...` - 查询标签掌握度（分页/筛选/排序）
- `POST /user/:uuid/mastery/events?operator_uuid=...` - 提交学习事件（写入掌握度）
- `DELETE /user/:uuid/mastery/questions/:number?operator_uuid=...` - 重置某题掌握度
- `DELETE /user/:uuid/mastery/tags?tag=xxx&operator_uuid=...` - 重置某标签掌握度

**更新用户信息** `PUT /user/:uuid?operator_uuid=...`
```json
{
  "nickname": "新昵称",
  "email": "a@b.com",
  "avatar_url": "https://..."
}
```

**查询题目掌握度** `GET /user/:uuid/mastery/questions?operator_uuid=...&pageIdx=1&pageSize=20&min_mastery=0.5&sort=mastery&order=desc`

**提交学习事件** `POST /user/:uuid/mastery/events?operator_uuid=...`
```json
{
  "question_number": 1001,
  "accepted": true
}
```

**常见错误**
- `403`: 无权限（operator_uuid 不是本人且无 admin 权限）
- `404`: 用户不存在 / 题目不存在
- `400`: 参数错误（例如 status 非 active/disabled）

### 题目管理
- `GET /question/` - 获取题目列表（按题目编号排序）
  可以通过 q 参数来搜索，例如 /question?q=12
- `GET /question/:number` - 通过题目编号获取单个题目详情
- `POST /question/` - 创建题目
- `POST /question/:number` - 更新题目（使用题目编号）

#### POST 接口 JSON 格式

**创建题目** `POST /question/`

完整版示例（包含所有可选字段）：
```json
{
    "question_id": "p1001",
    "title": "两数之和",
    "content": "给定一个整数数组 nums 和一个整数目标值 target，请你在该数组中找出和为目标值 target 的那两个整数，并返回它们的数组下标。",
    "difficulty": "简单",
    "time_limit": 1000,
    "memory_limit": 128,
    "tags": "数组,哈希表",
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
- `question_id`: 题目ID（必填）
- `title`: 题目标题（必填）
- `content`: 题目描述（必填）
- `difficulty`: 难度等级（必填）
- `time_limit`: 时间限制，单位毫秒（可选，默认1000）
- `memory_limit`: 内存限制，单位MB（可选，默认128）
- `tags`: 题目标签，逗号分隔（可选）
- `status`: 题目状态（可选，默认"published"，可选值：published/hint）
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

**获取最近题目** `GET /question/new`

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

**删除分类** `DELETE /category/delete`
```json
{
    "id": 1,
    "uuid": "xxx" // 账号uuid
}
```

### 关系管理
- `GET /relation/` - 获取关系列表

**调用示例**：`GET /relation/`

**响应示例**：
```json
{
  "result": [
    {
      "id": 1,
      "source_id": 10,
      "target_id": 12,
      "relation": "PREREQUISITE"
    }
  ]
}
```

### 节点管理
- `GET /node/` - 获取节点列表

**调用示例**：`GET /node/`

**响应示例**：
```json
{
  "result": [
    {
      "id": 1,
      "name": "数组",
      "type": "tag",
      "content": "数组相关基础概念"
    }
  ]
}
```

### 知识图谱（Neo4j）
说明：以下接口仅在 Neo4j 连接初始化成功时才会注册（否则 `/graph/*` 不可用）。题目标识使用 `question_number`（如 1001）。

- `POST /graph/questions/:number/sync` - 同步题目节点 + 标签节点/边 + 同标签题目边（写入 Neo4j）

说明：该接口会将题目写入 `(:Question)`，并根据题目 `tags` 自动：
- 创建/更新 `(:Skill)` 节点
- 建立 `(Question)-[:HAS_SKILL]->(Skill)` 边（auto=true）
- 建立题目间 `(:Question)-[:TAG_SIMILAR]->(:Question)` 边（auto=true，weight=共同标签数量）

**调用示例**：`POST /graph/questions/1001/sync`

**响应示例**：
```json
{ "message": "题目、标签及同标签关系同步成功" }
```

- `POST /graph/relations` - 创建题目关系边

**请求示例**：
```json
{
  "from_question": 1001,
  "to_question": 1002,
  "relation_type": "PREREQUISITE",
  "weight": 0.9,
  "description": "1001 是 1002 的前置基础"
}
```

**响应示例**：
```json
{ "message": "关系创建成功" }
```

- `DELETE /graph/relations` - 删除题目关系边

**请求示例**：
```json
{
  "from_question": 1001,
  "to_question": 1002,
  "relation_type": "PREREQUISITE"
}
```

**响应示例**：
```json
{ "message": "关系删除成功" }
```

- `GET /graph/questions/:number/prerequisites` - 查询前置题

**调用示例**：`GET /graph/questions/1002/prerequisites`

**响应示例**：
```json
{
  "question_number": 1002,
  "prerequisites": [
    {
      "question_number": 1001,
      "title": "两数之和",
      "difficulty": "简单",
      "tags": "数组,哈希表",
      "status": "published",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

- `GET /graph/questions/:number/next` - 查询可进阶题

**调用示例**：`GET /graph/questions/1001/next`

**响应示例**：
```json
{
  "question_number": 1001,
  "next_questions": [
    {
      "question_number": 1002,
      "title": "三数之和",
      "difficulty": "中等",
      "tags": "数组,双指针",
      "status": "published",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

- `GET /graph/questions/:number/recommendations?limit=5` - 获取推荐题目

说明：推荐来源包含三类，会通过 `relation_type` 与 `reason` 区分：
- `NEXT_LEVEL` / `SIMILAR`：题目之间已有关系边
- `TAG`：同标签推荐（基于 `(:Question)-[:HAS_SKILL]->(:Skill)<-[:HAS_SKILL]-(:Question)`）
- `TAG_CO_OCCUR`：共现标签推荐（基于 `(:Skill)-[:SKILL_CO_OCCUR]-(:Skill)` 再回到题目）

**调用示例**：`GET /graph/questions/1001/recommendations?limit=5`

**响应示例**：
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
    },
    {
      "question_number": 1010,
      "title": "两数之和 II",
      "difficulty": "简单",
      "score": 2,
      "reason": "同标签: 2 个",
      "relation_type": "TAG"
    },
    {
      "question_number": 1020,
      "title": "最长上升子序列",
      "difficulty": "中等",
      "score": 5,
      "reason": "共现标签",
      "relation_type": "TAG_CO_OCCUR"
    }
  ]
}
```

- `GET /graph/path?start=1001&end=1005` - 查找学习路径（最短路径）

**调用示例**：`GET /graph/path?start=1001&end=1005`

**响应示例**：
```json
{
  "start_question": 1001,
  "end_question": 1005,
  "path": [1001, 1002, 1005],
  "total_weight": 1.7,
  "path_length": 2
}
```

接口说明：

GET /graph/node
响应字段：
questions: 全部题目节点（QuestionNode）
skills: 全部技能/标签节点（SkillNode）
question_relations: 题目-题目关系（PREREQUISITE/NEXT_LEVEL/SIMILAR/CATEGORY 等）
question_skill_relations: 题目-技能关系（HAS_SKILL）
skill_relations: 技能-技能关系（SKILL_CO_OCCUR/SKILL_SUBSUMES）
edges: 统一边列表（from/to 使用 Q:1001、S:array 形式，便于前端直接绘图）
count: 题目数量
skill_count: 技能数量
edge_count: 边数量

### 智能推荐
- `POST /api/v1/recommendations` - 智能题目推荐（基于用户掌握度 + Neo4j 知识图谱）

**请求示例（默认模式）**：
```json
{
  "user_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "mode": "default",
  "limit": 20,
  "constraints": {
    "mastery_threshold": 0.7,
    "difficulty_tolerance": 1,
    "max_depth": 6
  }
}
```

**请求示例（目标模式）**：
```json
{
  "user_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "mode": "target",
  "target_question": 1005
}
```

**响应示例**：
```json
{
  "recommendations": [
    {
      "question_id": "1002",
      "score": 0.83,
      "breakdown": {
        "improvement": 0.9,
        "consolidation": 0,
        "diversity": 0.6
      },
      "explanation": {
        "path": ["1001→1002"],
        "edge_types": ["NEXT_LEVEL"],
        "edge_weights": [1],
        "confidence": 0.83
      }
    }
  ]
}
```

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

**删除测试用例** `DELETE /testcase/:id`
```json
{
    "id": 1
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

### OJ首页相关路由
- `GET /overview/getHomeText` - 获取OJ首页文本
- `POST /overview/updateHomeText` - 更新OJ首页文本

#### POST 接口 JSON 格式

**更新OJ首页文本** `POST /overview/updateHomeText`
```json
{
    "home_text": "更新后的OJ首页文本"
}
```

### OSS文件管理
#### A. 文件上传接口
*   **URL**: `POST /oss/upload`
*   **Content-Type**: `multipart/form-data`
*   **参数**:
    *   `file`: (必须) 要上传的文件。
    *   `path`: (可选) 目标目录前缀，如 `problems/1001/`。默认为 `uploads/`。
*   **响应示例**:
    ```json
    {
        "message": "上传成功",
        "key": "problems/1001/uuid-filename.txt",
        "size": 1024,
        "etag": "..."
    }
    ```

前端对接流程

1. 第一步：获取上传链接
   
   - 请求： GET /oss/upload-url?filename=input.txt&path=problems/1001/
   - 响应：
     ```
     {
         "url": "http://minio:9000/
         bucket/problems/1001/xxx.
         txt?Signature=...",
         "key": "problems/1001/xxx.
         txt"
     }
     ```
2. 第二步：前端直传 MinIO
   
   - 前端使用 PUT 方法，将文件二进制流直接发送到 url 。
   - 注意 ：不要带自定义 Header（除非后端签名时加了）， Content-Type 最好设为文件真实类型或 application/octet-stream 。
3. 第三步：保存元数据（可选但推荐）
   
   - 上传成功后，前端将 key 发送给业务后端（比如创建题目接口），后端将这个 key 存入数据库。

#### B. 目录结构展示接口
*   **URL**: `GET /oss/files`
*   **参数**:
    *   `prefix`: (可选) 目录路径，如 `problems/`。
    *   `recursive`: (可选) `true` 或 `false`。设为 `false` 时模拟文件夹结构。
*   **响应示例**:
    ```json
    {
        "prefix": "problems/",
        "objects": [
            {
                "key": "problems/1001/",
                "size": 0,
                "is_dir": true,
                "last_modified": "..."
            },
            {
                "key": "problems/readme.txt",
                "size": 500,
                "is_dir": false,
                "last_modified": "..."
            }
        ]
    }
    ```

如何使用（前端直传 OSS + commit）

- 约定 key 命名（推荐）： problems/{questionNumber}/{caseNo}.in 、 problems/{questionNumber}/{caseNo}.out
- 前端用 GET /oss/upload-url 获取预签名 PUT（你们项目里已存在该接口），把 .in/.out 直接上传到 OSS
- 上传成功后调用后端落库：
```
POST /testcase/oss/commit
{
  "question_number": 1001,
  "input_key": "problems/1001/1.in",
  "output_key": "problems/1001/1.out",
  "is_hidden": true
}
```

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
