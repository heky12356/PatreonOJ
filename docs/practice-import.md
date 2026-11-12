# 增量导入题目与学习图谱

`cmd/import_practice` 用于向已经初始化的站点追加题目、测试数据和明确的题目关系。它不会重建表，也不会创建用户、模拟提交或修改已有的做题进度。请勿在有真实数据的环境运行会重置数据库的 `cmd/seed_demo`。

## 准备题包

题包是 UTF-8 JSON 文件。隐藏测试数据也在其中，请放在私有目录，不要提交到公开仓库。导入第三方题目之前，应确认题面和测试数据允许使用。

```json
{
  "id": "my-practice-v1",
  "problems": [
    {
      "id": "KG9001",
      "title": "两个整数相加",
      "content": "## 题目描述\n输入两个整数，输出它们的和。\n\n## 输入格式\n一行两个整数。\n\n## 输出格式\n输出它们的和。\n\n## 数据范围\n每个整数绝对值不超过 1000000。",
      "difficulty": "简单",
      "tags": ["基础", "输入输出"],
      "time_limit": 2000,
      "memory_limit": 256,
      "cases": [
        {"input": "1 2\n", "output": "3\n", "hidden": false},
        {"input": "-7 3\n", "output": "-4\n", "hidden": true}
      ]
    }
  ],
  "edges": []
}
```

每题至少包含一个公开样例和一个隐藏测试。时间限制单位为毫秒，内存限制单位为 MB。正式题包应包含完整题面、样例解释、数据范围，以及边界、随机和规模测试。应使用可信的参考程序验证答案。

关系的端点使用业务题号（如 `KG9001`），而非数据库主键。可以引用同一题包中的题目，也可以引用数据库中唯一存在、已发布的题目：

```json
{"from": "KG9001", "to": "KG9002", "type": "PREREQUISITE"}
```

三种关系分别是：

| type | 含义和方向 |
| --- | --- |
| PREREQUISITE | 基础题 → 依赖它的题目，用于卡住时回退补基础 |
| NEXT_LEVEL | 基础题 → 进阶题，用于通过后的下一步推荐 |
| SIMILAR | 同类变式；需要双向扩展时显式提供两个方向 |

同一条学习路线通常同时提供 PREREQUISITE 和 NEXT_LEVEL 两条关系。导入器只追加题包中指定的关系，不运行自动关系重建，也不删除其他关系。

## 在 Docker Compose 部署中导入

以下命令在从 GitHub 克隆得到的 `PatreonOJ` 目录中执行，沿用部署指南中已经配置好的 Compose 环境。先备份 MySQL 和对象存储，再导入；期间串行执行导入任务，避免与后台编辑同一批题目同时进行。

在有 Go 工具链的机器上构建与服务器架构一致的 Linux 程序。以下示例适用于 amd64 服务器：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o patreonoj-import-practice ./cmd/import_practice
```

把程序和私有题包上传到服务器的受限目录，然后复制到 API 容器。下面假定服务器当前目录有这两个文件：

```bash
api_container=$(bash deploy/compose.sh ps -q api)
docker cp patreonoj-import-practice "$api_container:/tmp/patreonoj-import-practice"
docker cp private-pack.json "$api_container:/tmp/private-pack.json"
bash deploy/compose.sh exec -T --user root api sh -c \
  'chmod 755 /tmp/patreonoj-import-practice && chown 10001:10001 /tmp/private-pack.json && chmod 600 /tmp/private-pack.json'
```

先执行数据库预检，不添加 `-apply` 不会写入题目或上传文件：

```bash
bash deploy/compose.sh exec -T api /tmp/patreonoj-import-practice \
  -config /app/config.yaml -file /tmp/private-pack.json
```

预检通过后执行导入，并保存输出清单到私有位置：

```bash
bash deploy/compose.sh exec -T api /tmp/patreonoj-import-practice \
  -config /app/config.yaml -file /tmp/private-pack.json -apply
```

程序复用 API 容器内的 MySQL、对象存储配置；不需要向主机公网开放这些服务。数据库必须已经由应用初始化，存储桶必须已存在。

重复导入完全相同的题包会跳过已有题目和关系。若题号与现有题目冲突，或内容、测试文件清单有变化，预检会报错，不覆盖现有数据。需要更改已导入题目时，使用后台编辑或为新版本分配新题号，不应靠更换题包版本绕过冲突检查。

数据库写入在事务中执行，上传失败时回滚本次数据库写入。对象存储不支持与 MySQL 共同提交事务：失败后可能留下本次版本前缀下未被引用的对象；可以修复连接后重试相同题包，或者核对数据库引用和备份后清理这些对象。已有题目的对象不会被导入器覆盖。

导入后，打开题库和任意新题的“学习图谱”，检查节点、关系类型、深度切换和题目跳转，再使用普通账号提交参考程序确认 AC。连续未通过与通过后的推荐应分别展示补基础和进阶路线。验证完成后可删除容器 `/tmp` 下的程序和题包，保留私有题包、导入清单及备份。

## 算法学习路线题库

`algorithm-paths-v1` 包含 24 道题，每题 10 组测试（2 组公开、8 组隐藏），共 70 条显式关系。题目采用经典算法主题的独立题面与测试数据，没有导入其他站点的题面或用户记录。题包及参考答案由部署者私下保存。

| 题号 | 题目 | 主要知识点 |
| --- | --- | --- |
| KG1001 | 序列求和 | 数组、整数范围 |
| KG1002 | 区间和查询 | 前缀和 |
| KG1003 | 二维区域和 | 二维前缀和 |
| KG1004 | 和为 K 的连续子段 | 前缀和、哈希表 |
| KG1005 | 最长无重复连续片段 | 双指针、滑动窗口 |
| KG1101 | 有序位置查找 | 二分查找 |
| KG1102 | 最小载重 | 二分答案、贪心 |
| KG1103 | 最长递增子序列 | 动态规划、二分查找 |
| KG1201 | 括号配对检查 | 栈 |
| KG1202 | 最大括号嵌套层数 | 栈、计数 |
| KG1203 | 重复式字符串解码 | 栈、嵌套结构 |
| KG2001 | 台阶方案数 | 动态规划入门 |
| KG2002 | 不相邻元素的最大和 | 状态转移 |
| KG2003 | 最大连续子段和 | 状态转移 |
| KG2004 | 最少硬币 | 完全背包 |
| KG2005 | 背包挑选物品 | 01 背包 |
| KG2006 | 编辑距离 | 二维动态规划 |
| KG3000 | 图中邻居计数 | 图的表示 |
| KG3001 | 从起点能到多少点 | 图的搜索 |
| KG3002 | 连通块数量 | 搜索、连通性 |
| KG3003 | 迷宫最短步数 | 广度优先搜索 |
| KG3004 | 课程安排 | 拓扑排序 |
| KG3005 | 正权最短路径 | 堆、最短路 |
| KG3006 | 最小连接费用 | 并查集、最小生成树 |

可从“序列求和”“区间和查询”观察前缀和分支；从“台阶方案数”观察动态规划分支；从“图中邻居计数”观察搜索、最短路和最小生成树分支。括号检查与嵌套层数、两类背包等题通过同类关系相连，方便检查三种边类型的显示与筛选。
