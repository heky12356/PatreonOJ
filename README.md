# PatreonOJ

基于 Go（Gin + GORM）的在线判题系统，当前以 MySQL/SQLite 为主存储，支持题目管理、提交评测、测试用例管理、OSS 文件管理，以及基于用户做题表现的学习推荐（研究报告第二部分的最小实现）。

## 当前能力
- 用户：注册、登录、登出、用户信息查询与更新
- 题目：列表、创建、更新、删除、按 `id`/`question_id` 查询
- 提交：提交代码、查询评测结果、按题目/用户分页查询提交记录
- 测试用例：单条/批量创建、按题目查询、OSS commit、更新删除
- 学习推荐：基于用户在某题上的尝试/通过情况，返回推荐题目列表
- 概览：首页文案、公告的读取和更新
- OSS：上传、预签名 URL、文件列表

## 关于图数据库（Neo4j）
- 目前仅保留初始化流程（启动时可做 `InitGraph`）。
- 面向前端展示和图查询的 `/graph/*` 路由已下线，待后续重新设计数据结构后再开放。

## 目录结构
- `cmd/PatreonOJ`: 主程序入口
- `internal/controllers`: HTTP 控制器层
- `internal/services`: 业务逻辑层
- `internal/models`: 数据模型
- `internal/routers`: 路由注册
- `internal/graph`: Neo4j 客户端与初始化逻辑（暂不对外暴露路由）
- `doce/openapi.yaml`: 当前 API 文档（OpenAPI 3.0）

## 快速开始
### 1. 安装依赖
```bash
go mod tidy
```

### 2. 配置
准备 `config.yaml`，至少配置：
- `server`
- `database`
- `judge`
- `oss`
- `graph_database`（可选，仅用于初始化）

### 3. 启动服务
```bash
go run cmd/PatreonOJ/main.go
```

默认地址：`http://localhost:8080`

## 主要接口（按当前 router）

### 用户 `/user`
- `GET /user/`
- `POST /user/login`
- `POST /user/register`
- `POST /user/logout`
- `GET /user/solves/:uuid`
- `GET /user/solve/?uuid=&question_number=`
- `GET /user/:uuid`
- `PUT /user/:uuid?operator_uuid=`

### 提交查询 `/api`
- `GET /api/problems/:question_number/submissions`
- `GET /api/problems/id/:question_id/submissions`
- `GET /api/users/:user_id/submissions`

### 学习推荐 `/learning`
- `GET /learning/users/:uuid/questions/:question_id/recommendations?limit=5`

返回核心字段：
- `attempts`：尝试次数
- `pass_count`：通过次数
- `pass_rate`：通过率
- `state`：`new | learning | challenged | mastered`
- `recommendations`：推荐题列表（含 `edge_type` 与 `reason`）

### 题目 `/question`
- `GET /question/`
- `GET /question/new`
- `GET /question/id/:question_id`
- `GET /question/:number`
- `POST /question/`
- `POST /question/id/:question_id`
- `POST /question/:number`（兼容）
- `DELETE /question/delete`

### 提交 `/submission`
- `POST /submission/`
- `GET /submission/:id`

### 测试用例 `/testcase`
- `GET /testcase/`
- `GET /testcase/question/:number`
- `GET /testcase/question/id/:question_id`
- `GET /testcase/:id`
- `POST /testcase/`
- `POST /testcase/batch`
- `POST /testcase/oss/commit`
- `PUT /testcase/:id`
- `DELETE /testcase/:id`

### 概览 `/overview`
- `GET /overview/getHomeText`
- `POST /overview/updateHomeText`
- `GET /overview/getAnnouncement`
- `POST /overview/updateAnnouncement`

### OSS `/oss`
- `POST /oss/upload`
- `GET /oss/upload-url`
- `GET /oss/files`

## OpenAPI
- 文档文件：`doce/openapi.yaml`
- 可导入 Swagger Editor / Apifox / Postman（OpenAPI 3.0）

## 测试
```bash
go test ./...
```
