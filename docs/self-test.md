# 在线自测与标签图谱

题目详情的代码编辑器支持“运行自测”和“正式提交”。自测使用自定义标准输入，或加载最多 5 个公开样例，可以对照预期输出，并查看标准输出、错误输出、用时、内存和退出码。`Ctrl/Cmd + Enter` 运行自测。这里是输入输出式运行，不提供断点或单步调试。

自测只调用配置的远程 go-judge 沙箱，不在 API 宿主机执行代码，不读取隐藏测试数据，不创建正式提交、通过记录或技能掌握记录。正常运行显示 `OK`，输出对照显示 match/mismatch，不能代替正式评测的 AC/WA。C++、Python、Java、Go 共用现有编译和资源限制逻辑；编译失败显示 CE，运行异常显示 RE/TLE 等。

接口均以站点同源 `/api` 为前缀：

| 方法和路径 | 用途与权限 |
| --- | --- |
| `GET /question/:id/samples` | 获取公开样例；公开题可匿名访问，草稿仅管理员可见，始终不包含隐藏用例 |
| `POST /debug/runs` | 登录后创建自测；请求含 `question_pk/language/code/input`，可选 `expected_output` |
| `GET /debug/runs/:id` | 仅创建者读取结果；不存在和他人任务均返回 404 |
| `DELETE /debug/runs/:id` | 仅创建者停止任务；已完成任务返回现有结果 |
| `GET /learning/tags` | 公开题的精确标签及题目数量 |
| `GET /learning/graph?tag=数组&page=1&page_size=60&q=` | 标签内的题目关系分页子图，支持业务编号/标题筛选 |

任务状态为 pending/running/completed/cancelled，结果另含 verdict、stdout、stderr、runtime_ms、memory_kb、exit_code、comparison。代码最多 256 KiB，输入和预期输出分别最多 64 KiB。每个用户最多一个活动任务，当前 API 单个自测 worker、4 个等待位；繁忙返回 429，不无限排队。运行限制根据题目配置收敛到 100–5000 ms、64–512 MiB；编译加运行的任务上下文最多 120 秒。输出也受现有沙箱文件限制约束。

自测结果保存在 API 内存，完成后保留最多 15 分钟，最多 64 条；容量满时提前淘汰最早完成的结果。重启后丢失，适合调试，不作为提交档案。未来增加 API 副本前，需要任务路由或共享任务存储。

Header 的“知识图谱”进入标签目录：选择标签看本标签的题目关系，再打开单题图谱查看标签外的前置和进阶题。标签是筛选入口，图中节点仍是题目，不是人为定义的知识点先修图。图谱容量、维护和缓存说明见 [图谱性能与扩容](graph-performance.md)。
