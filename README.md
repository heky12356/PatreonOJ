# PatreonOJ

从 GitHub 拉取两个仓库、Docker Compose 构建部署、MySQL 配置、首次管理员授权、备份与升级见 [部署指南](部署指南.md)。生产使用 MySQL、Caddy 和独立判题容器；本地开发默认监听 `127.0.0.1:8080`。

向已有站点追加题目、测试数据与图谱关系，见 [增量导入与算法学习路线](docs/practice-import.md)。

题目编辑器的自定义输入、公开样例及结果权限见 [在线自测与标签图谱](docs/self-test.md)。图谱的实测、稀疏建边、索引、缓存和旧关系安全重建见 [性能与扩容说明](docs/graph-performance.md)。

基于 Go（Gin + GORM）的在线判题系统，当前以 MySQL/SQLite 为主存储，支持题目管理、提交评测、测试用例管理、OSS 文件管理，以及基于用户做题表现的学习推荐（研究报告第二部分的最小实现）。

## 当前能力
- 用户：注册、登录、登出、用户信息查询与更新
- 题目：列表、创建、更新、删除、按 `id`/`question_id` 查询
- 题库筛选：`GET /question/?tag=数组&difficult=简单&q=求和` 可组合精确标签、难度与关键词；空标签表示全部，分页与总数基于同一筛选条件，未登录用户仅可查询已发布题目
- 提交：提交代码、查询评测结果、按题目/用户分页查询提交记录
- 测试用例：单条/批量创建、按题目查询、OSS commit、更新删除
- 学习推荐：基于用户在某题上的尝试/通过情况、题目关系和技能掌握度，返回推荐题、题目关系图和学习反馈
- 概览：首页文案、公告的读取和更新
- OSS：上传、预签名 URL、文件列表

生产启动时会幂等初始化 `admin`、`moderator`、`teacher`、`student`、`user` 基础角色，管理面板通过角色目录接口加载选项。角色权限和直接权限分别保存，账号的实际权限为两者的合并结果。

## 关于图数据库（Neo4j）
- 默认关闭，通过 `graph_database.enabled: true` 开启初始化同步；当前网页关系图和学习推荐不依赖 Neo4j。
- 面向前端展示和图查询的 `/graph/*` 路由已下线，待后续重新设计数据结构后再开放。

## 目录结构
- `cmd/PatreonOJ`: 主程序入口
- `internal/Controllers`: HTTP 控制器层（目录大小写与 Git 一致）
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
首次运行复制 `default_config.yaml` 为 `config.yaml`，填写真实对象存储凭据，至少配置：
- `server`
- `database`
- `judge`
- `oss`
- `graph_database`（可选，仅用于初始化）

### 3. 启动服务
```bash
go run cmd/PatreonOJ/main.go
```

默认地址：`http://127.0.0.1:8080`；`GET /healthz` 验证 API 与数据库连接。

## 主要接口（按当前 router）

### 用户 `/user`
- `GET /user/`
- `POST /user/login`
- `POST /user/register`
- `POST /user/logout`
- `GET /user/solves/:uuid`
- `GET /user/roles`（管理员读取实际可用角色）
- `GET /user/me`
- `GET /user/me/problem-status?ids=1,2`（仅按当前会话查询个人题目状态）
- `GET /user/:id/home`（公开主页；遵守用户的 `show_activity` 设置）
- `GET /user/:id`（数字 ID / UUID，仅本人和管理员）
- `PUT /user/:id`（数字 ID / UUID，仅本人和管理员）
- `PUT /user/update/:id`（旧路径兼容）
- `GET /user/:id/public`（匿名公开昵称和头像，不返回邮箱）

### 提交查询 `/api`
- `GET /api/problems/:id/submissions`（题目数据库主键）
- `GET /api/users/:user_id/submissions`

### 学习推荐与反馈闭环 `/learning`
- `GET /learning/users/:uuid/feedback?limit=5`
- `GET /learning/users/:uuid/questions/:id/recommendations?limit=5`
- `GET /learning/questions/:id/graph?depth=3`

推荐闭环由三部分组成：
- 题目关系图：`/learning/questions/:id/graph` 基于 `question_relation_edges` 返回 `nodes`、`edges`、`tree`，用于展示当前题目的前置、进阶和相似题关系。
- 下一题推荐：`/learning/users/:uuid/questions/:id/recommendations` 根据用户在该题上的提交和通过情况判断 `new | learning | challenged | mastered`，再按 `PREREQUISITE`、`NEXT_LEVEL`、`SIMILAR` 和标签匹配返回推荐题。
- 学习反馈：`/learning/users/:uuid/feedback` 汇总 `user_skill_mastery`、已解题和未解题，返回薄弱技能、已掌握技能、下一步题目和正向反馈文案。

注意：这里的 `:id` 是题目表主键 `question.id`，不是业务题号字段 `question.question_id`。演示数据可通过 `go run ./cmd/seed_demo` 初始化；该命令会重建当前配置指向的数据库，适合演示环境使用。

返回核心字段：
- `attempts`：尝试次数
- `pass_count`：通过次数
- `pass_rate`：通过率
- `state`：`new | learning | challenged | mastered`
- `recommendations`：推荐题列表（含 `edge_type` 与 `reason`）
- `weak_skills` / `mastered_skills`：用户学习反馈中的薄弱和已掌握知识点
- `next_questions` / `positive_feedback`：反馈闭环中的下一步题目和鼓励文案

### 题目 `/question`
- `GET /question/`
- `GET /question/new`
- `GET /question/:id`（题目数据库主键）
- `POST /question/`
- `POST /question/:id`（更新）
- `DELETE /question/delete`

提交详情通过 `data.source_code` 返回源码；`data.code` 兼容旧客户端，顶层 `code` 保留响应状态码。提交接口保留任务的 `status`，并以独立 `verdict` 返回 `AC`、`WA`、`RE`、`TLE`、`MLE`、`OLE`、`CE`、`SE` 或排队状态。提交列表支持按 `verdict` 筛选，并返回用户名。公开主页默认展示做题详情，用户可在个人设置修改 `show_activity`；关闭后其他用户不能查看其做题记录、统计或提交代码。

### 提交 `/submission`
- `POST /submission/`
- `GET /submission/:id`

### 测试用例 `/testcase`
- `GET /testcase/`
- `GET /testcase/question/:id`
- `GET /testcase/:id`
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
