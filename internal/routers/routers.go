package routers

import (
	"dachuang/internal/Controllers/admin"
	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
)

func RoutersInit(r *gin.Engine) {
	// 用户相关路由
	userRouter := r.Group("/user")
	{
		userCtrl := admin.NewUserController(models.DB)
		userRouter.GET("/", userCtrl.Index)
		userRouter.POST("/login", userCtrl.Login)
		userRouter.POST("/register", userCtrl.Register)
		userRouter.POST("/logout", userCtrl.Logout)
	}

	// 题目相关路由
	questionRouter := r.Group("/question")
	{
		questionCtrl := &admin.QuestionController{}
		questionRouter.GET("/", questionCtrl.Index)
		questionRouter.GET("/:number", questionCtrl.Show)        // 通过题目编号获取单个题目
		questionRouter.POST("/", questionCtrl.Store)
		questionRouter.POST("/:number", questionCtrl.Update)     // 改为使用题目编号
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
		testCaseRouter.GET("/", testCaseCtrl.Index)                    // 获取测试用例列表
		testCaseRouter.GET("/question/:number", testCaseCtrl.GetByQuestion) // 根据题目编号获取测试用例
		testCaseRouter.GET("/:id", testCaseCtrl.Show)                  // 获取单个测试用例详情
		testCaseRouter.POST("/", testCaseCtrl.Store)                   // 添加单个测试用例
		testCaseRouter.POST("/batch", testCaseCtrl.BatchStore)         // 批量添加测试用例
		testCaseRouter.PUT("/:id", testCaseCtrl.Update)               // 更新测试用例
		testCaseRouter.DELETE("/:id", testCaseCtrl.Delete)            // 删除测试用例
	}
}