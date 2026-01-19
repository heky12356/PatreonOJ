package routers

import (
	"context"
	"log"

	"dachuang/internal/Controllers"
	"dachuang/internal/Controllers/admin"
	"dachuang/internal/graph"
	"dachuang/internal/models"
	"dachuang/internal/oss"

	"github.com/gin-gonic/gin"
)

func RoutersInit(r *gin.Engine, ossClient *oss.OSS, graphService *graph.QuestionGraphService) {
	userCtrl := admin.NewUserController(models.DB, ossClient, graphService)

	// 用户相关路由
	userRouter := r.Group("/user")
	{
		userSolveCtrl := admin.NewUserSolveController(models.DB)
		userRouter.GET("/", userCtrl.Index)
		userRouter.POST("/login", userCtrl.Login)
		userRouter.POST("/register", userCtrl.Register)
		userRouter.POST("/logout", userCtrl.Logout)
		userRouter.GET("/solves/:uuid", userSolveCtrl.Index)
		userRouter.GET("/solve/", userSolveCtrl.Show)

		userRouter.GET("/:uuid", userCtrl.Show)
		userRouter.PUT("/:uuid", userCtrl.Update)
		userRouter.GET("/:uuid/mastery/questions", userCtrl.ListQuestionMastery)
		userRouter.GET("/:uuid/mastery/tags", userCtrl.ListTagMastery)
		userRouter.POST("/:uuid/mastery/events", userCtrl.SubmitMasteryEvent)
		userRouter.DELETE("/:uuid/mastery/questions/:number", userCtrl.DeleteQuestionMastery)
		userRouter.DELETE("/:uuid/mastery/tags", userCtrl.DeleteTagMastery)
	}

	apiV1Router := r.Group("/api/v1")
	{
		apiV1Router.POST("/recommendations", userCtrl.GetRecommendationsV1)
	}

	apiRouter := r.Group("/api")
	{
		apiRouter.GET("/problems/:question_number/submissions", userCtrl.ListProblemSubmissions)
		apiRouter.GET("/users/:user_id/submissions", userCtrl.ListUserSubmissions)
	}

	// 题目相关路由
	questionRouter := r.Group("/question")
	{
		questionCtrl := admin.NewQuestionController(models.DB)
		questionRouter.GET("/", questionCtrl.Index)
		questionRouter.GET("/new", questionCtrl.GetNewProblems)
		questionRouter.GET("/id/:question_id", questionCtrl.ShowByQuestionID) // 通过自定义question_id获取单个题目
		questionRouter.GET("/:number", questionCtrl.Show)                    // 通过题目编号获取单个题目
		questionRouter.POST("/", questionCtrl.Store)
		questionRouter.POST("/:number", questionCtrl.Update) // 改为使用题目编号
		questionRouter.DELETE("/delete", questionCtrl.DeleteProblem)
	}

	// 分类相关路由
	categoryRouter := r.Group("/category")
	{
		categoryCtrl := &admin.CategoryController{}
		categoryRouter.GET("/", categoryCtrl.Index)
		categoryRouter.POST("/", categoryCtrl.Store)
		categoryRouter.POST("/:id", categoryCtrl.Update)
		categoryRouter.DELETE("/delete/:id", categoryCtrl.Delete)
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
		submissionRouter.POST("/", userCtrl.SubmitCode)
		submissionRouter.GET("/:id", userCtrl.GetSubmissionResult)
	}

	// 测试用例相关路由
	testCaseRouter := r.Group("/testcase")
	{
		testCaseCtrl := admin.NewTestCaseController(ossClient)
		testCaseRouter.GET("/", testCaseCtrl.Index)                         // 获取测试用例列表
		testCaseRouter.GET("/question/:number", testCaseCtrl.GetByQuestion) // 根据题目编号获取测试用例
		testCaseRouter.GET("/:id", testCaseCtrl.Show)                       // 获取单个测试用例详情
		testCaseRouter.POST("/", testCaseCtrl.Store)                        // 添加单个测试用例
		testCaseRouter.POST("/batch", testCaseCtrl.BatchStore)              // 批量添加测试用例
		testCaseRouter.POST("/oss/commit", testCaseCtrl.OSSCommit)
		testCaseRouter.PUT("/:id", testCaseCtrl.Update)    // 更新测试用例
		testCaseRouter.DELETE("/:id", testCaseCtrl.Delete) // 删除测试用例
	}

	// 图数据库相关路由
	if graphService != nil {
		graphRouter := r.Group("/graph")
		{
			if err := graphService.InitGraph(context.Background(), models.DB); err != nil {
				log.Printf("Warning: Failed to init graph: %v", err)
			}
			graphCtrl := Controllers.NewGraphController(models.DB, graphService)
			// 获取所有题目节点
			graphRouter.GET("/node", graphCtrl.ListQuestions)
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

	// OJ首页相关路由
	ojOverViewRouter := r.Group("/overview")
	{
		ojOverViewCtrl := Controllers.NewOjOverViewController(models.DB)
		ojOverViewRouter.GET("/getHomeText", ojOverViewCtrl.GetHomeText)
		ojOverViewRouter.POST("/updateHomeText", ojOverViewCtrl.UpdateHomeText)
		ojOverViewRouter.GET("/getAnnouncement", ojOverViewCtrl.GetAnnouncement)
		ojOverViewRouter.POST("/updateAnnouncement", ojOverViewCtrl.UpdateAnnouncement)
	}

	// OSS 相关路由
	if ossClient != nil {
		ossRouter := r.Group("/oss")
		{
			ossCtrl := admin.NewOSSController(ossClient)
			ossRouter.POST("/upload", ossCtrl.UploadFile)
			ossRouter.GET("/upload-url", ossCtrl.GetUploadURL)
			ossRouter.GET("/files", ossCtrl.ListFiles)
		}
	}
}
