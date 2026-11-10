package routers

import (
	controllers "patreonoj/internal/controllers"
	adminauth "patreonoj/internal/controllers/auth"
	adminlearning "patreonoj/internal/controllers/learning"
	adminquestion "patreonoj/internal/controllers/question"
	adminstorage "patreonoj/internal/controllers/storage"
	adminsubmission "patreonoj/internal/controllers/submission"
	admintestcase "patreonoj/internal/controllers/testcase"
	adminuser "patreonoj/internal/controllers/user"
	"patreonoj/internal/middleware"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	questionsvc "patreonoj/internal/services/question"

	"github.com/gin-gonic/gin"
)

func RoutersInit(r *gin.Engine, ossClient *oss.OSS) {
	userCtrl := adminuser.NewUserController(models.DB)
	authCtrl := adminauth.NewAuthController(models.DB)
	submissionCtrl := adminsubmission.NewSubmissionController(models.DB, ossClient)
	learningCtrl := adminlearning.NewLearningController(models.DB)
	edgeBuilder := questionsvc.NewAsyncQuestionEdgeBuilder(models.DB, 256)
	userSolveCtrl := adminuser.NewUserSolveController(models.DB)
	questionCtrl := adminquestion.NewQuestionController(models.DB, edgeBuilder)
	ojOverViewCtrl := controllers.NewOjOverViewController(models.DB)
	testCaseCtrl := admintestcase.NewTestCaseController(models.DB, ossClient)

	r.Use(middleware.LoadCurrentUser(models.DB))

	authUserRouter := r.Group("/user").Use(middleware.RequireAuth())
	{
		authUserRouter.GET("/", userCtrl.Index)
		authUserRouter.GET("/me", userCtrl.GetProfile)
		authUserRouter.POST("/logout", authCtrl.Logout)
		authUserRouter.GET("/solves/:uuid", userSolveCtrl.Index)
		authUserRouter.GET("/:id", userCtrl.Show)
		authUserRouter.PUT("/update/:id", userCtrl.Update)
	}

	userRouter := r.Group("/user")
	{
		userRouter.POST("/login", authCtrl.Login)
		userRouter.POST("/register", authCtrl.Register)
		// 获取用户详细
		userRouter.GET("/:id/public", userCtrl.GetProfilePublic)
	}

	// 查询提交记录
	apiRouter := r.Group("/api")
	{
		apiRouter.GET("/problems/:id/submissions", submissionCtrl.ListProblemSubmissions)
		apiRouter.GET("/users/:user_id/submissions", middleware.RequireAuth(), submissionCtrl.ListUserSubmissions)
	}

	learningRouter := r.Group("/learning")
	{
		learningRouter.GET("/users/:uuid/feedback", learningCtrl.GetUserFeedback)
		learningRouter.GET("/users/:uuid/questions/:id/recommendations", learningCtrl.RecommendByQuestion)
		learningRouter.GET("/questions/:id/graph", learningCtrl.GetQuestionGraph)
	}

	questionRouter := r.Group("/question")
	{
		questionRouter.GET("/", questionCtrl.Index)
		questionRouter.GET("/new", questionCtrl.GetNewProblems)
		questionRouter.GET("/:id", questionCtrl.Show)
	}

	questionAuthRouter := r.Group("/question").Use(middleware.RequirePermission("admin"))
	{
		questionAuthRouter.POST("/", questionCtrl.Store)
		questionAuthRouter.POST("/:id", questionCtrl.Update)
		questionAuthRouter.DELETE("/delete", questionCtrl.DeleteProblem)
	}

	submissionRouter := r.Group("/submission").Use(middleware.RequireAuth())
	{
		submissionRouter.POST("/", submissionCtrl.SubmitCode)
		submissionRouter.GET("/:id", submissionCtrl.GetSubmissionResult)
	}

	testCaseRouter := r.Group("/testcase").Use(middleware.RequirePermission("admin"))
	{
		testCaseRouter.GET("/", testCaseCtrl.Index)
		testCaseRouter.GET("/question/:id", testCaseCtrl.GetByQuestion)
		testCaseRouter.GET("/:id", testCaseCtrl.Show)
		testCaseRouter.POST("/oss/commit", testCaseCtrl.OSSCommit)
		testCaseRouter.PUT("/:id", testCaseCtrl.Update)
		testCaseRouter.DELETE("/:id", testCaseCtrl.Delete)
	}

	ojOverViewRouter := r.Group("/overview")
	{
		ojOverViewRouter.GET("/getHomeText", ojOverViewCtrl.GetHomeText)
		ojOverViewRouter.GET("/getAnnouncement", ojOverViewCtrl.GetAnnouncement)
	}

	ojAuthOverViewRouter := r.Group("/overview").Use(middleware.RequirePermission("admin"))
	{
		ojAuthOverViewRouter.POST("/updateHomeText", ojOverViewCtrl.UpdateHomeText)
		ojAuthOverViewRouter.POST("/updateAnnouncement", ojOverViewCtrl.UpdateAnnouncement)
	}

	if ossClient != nil {
		ossRouter := r.Group("/oss").Use(middleware.RequireAuth())
		{
			ossCtrl := adminstorage.NewOSSController(ossClient)
			ossRouter.POST("/upload", ossCtrl.UploadFile)
			ossRouter.GET("/upload-url", ossCtrl.GetUploadURL)
			ossRouter.GET("/files", ossCtrl.ListFiles)
		}
	}
}
