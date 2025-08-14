package routers

import (
	"log"

	"dachuang/internal/Controllers"
	"dachuang/internal/Controllers/admin"
	"dachuang/internal/config"
	"dachuang/internal/graph"
	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
)

func RoutersInit(r *gin.Engine) {
	// 初始化图数据库服务
	var graphService *graph.QuestionGraphService
	if config.GlobalConfig != nil {
		neo4jClient, err := graph.NewNeo4jClient(graph.Neo4jConfig{
			URI:      config.GlobalConfig.GraphDatabase.Neo4j.URI,
			Username: config.GlobalConfig.GraphDatabase.Neo4j.Username,
			Password: config.GlobalConfig.GraphDatabase.Neo4j.Password,
			Database: config.GlobalConfig.GraphDatabase.Neo4j.Database,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize Neo4j client: %v", err)
		} else {
			graphService = graph.NewQuestionGraphService(neo4jClient)
		}
	}

	// 用户相关路由
	userRouter := r.Group("/user")
	{
		userCtrl := admin.NewUserController(models.DB)
		userSolveCtrl := admin.NewUserSolveController(models.DB)

		userRouter.GET("/", userCtrl.Index)
		userRouter.POST("/login", userCtrl.Login)
		userRouter.POST("/register", userCtrl.Register)
		userRouter.POST("/logout", userCtrl.Logout)
		userRouter.GET("/solves/:uuid", userSolveCtrl.Index)
		userRouter.GET("/solve/", userSolveCtrl.Show)
	}

	// 题目相关路由
	questionRouter := r.Group("/question")
	{
		questionCtrl := &admin.QuestionController{}
		questionRouter.GET("/", questionCtrl.Index)
		questionRouter.GET("/:number", questionCtrl.Show) // 通过题目编号获取单个题目
		questionRouter.POST("/", questionCtrl.Store)
		questionRouter.POST("/:number", questionCtrl.Update) // 改为使用题目编号
	}

	// 分类相关路由
	categoryRouter := r.Group("/category")
	{
		categoryCtrl := &admin.CategoryController{}
		categoryRouter.GET("/", categoryCtrl.Index)
		categoryRouter.POST("/", categoryCtrl.Store)
		categoryRouter.POST("/:id", categoryCtrl.Update)
	}

	// 关系相关路由
	relationRouter := r.Group("/relation")
	{
		relationCtrl := &admin.RelationController{}
		relationRouter.GET("/", relationCtrl.Index)
	}

	// 节点相关路由
	nodeRouter := r.Group("/node")
	{
		nodeCtrl := &admin.NodeController{}
		nodeRouter.GET("/", nodeCtrl.Index)
	}

	// 提交相关路由
	submissionRouter := r.Group("/submission")
	{
		userCtrl := admin.NewUserController(models.DB)
		submissionRouter.POST("/", userCtrl.SubmitCode)
		submissionRouter.GET("/:id", userCtrl.GetSubmissionResult)
	}

	// 测试用例相关路由
	testCaseRouter := r.Group("/testcase")
	{
		testCaseCtrl := &admin.TestCaseController{}
		testCaseRouter.GET("/", testCaseCtrl.Index)                         // 获取测试用例列表
		testCaseRouter.GET("/question/:number", testCaseCtrl.GetByQuestion) // 根据题目编号获取测试用例
		testCaseRouter.GET("/:id", testCaseCtrl.Show)                       // 获取单个测试用例详情
		testCaseRouter.POST("/", testCaseCtrl.Store)                        // 添加单个测试用例
		testCaseRouter.POST("/batch", testCaseCtrl.BatchStore)              // 批量添加测试用例
		testCaseRouter.PUT("/:id", testCaseCtrl.Update)                     // 更新测试用例
		testCaseRouter.DELETE("/:id", testCaseCtrl.Delete)                  // 删除测试用例
	}

	// 图数据库相关路由
	if graphService != nil {
		graphRouter := r.Group("/graph")
		{
			graphCtrl := Controllers.NewGraphController(models.DB, graphService)
			// 题目同步
			graphRouter.POST("/questions/:number/sync", graphCtrl.SyncQuestion)

			// 关系管理
			graphRouter.POST("/relations", graphCtrl.CreateRelation)
			graphRouter.DELETE("/relations", graphCtrl.DeleteRelation)

			// 题目关系查询
			graphRouter.GET("/questions/:number/prerequisites", graphCtrl.GetPrerequisites)
			graphRouter.GET("/questions/:number/next", graphCtrl.GetNextLevelQuestions)
			graphRouter.GET("/questions/:number/recommendations", graphCtrl.GetRecommendations)

			// 学习路径
			graphRouter.GET("/path", graphCtrl.FindLearningPath) // ?start=1001&end=1005
		}
	}
}
