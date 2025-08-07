package routers

import (
	"dachuang/Controllers/admin"
	"dachuang/config"
	"dachuang/models"

	"github.com/gin-gonic/gin"
)

func RoutersInit(r *gin.Engine) {
	// 用户相关路由
	userRouter := r.Group("/user")
	{
		userCtrl := admin.NewUserController(models.DB, config.GlobalConfig.Judge.APIURL)
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
		questionRouter.POST("/", questionCtrl.Store)
		questionRouter.POST("/:id", questionCtrl.Update)
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
		userCtrl := admin.NewUserController(models.DB, config.GlobalConfig.Judge.APIURL)
		submissionRouter.POST("/", userCtrl.SubmitCode)
		submissionRouter.GET("/:id", userCtrl.GetSubmissionResult)
	}
}