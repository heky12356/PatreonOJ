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

	userRouter := r.Group("/user")
	{
		userSolveCtrl := adminuser.NewUserSolveController(models.DB)
		userRouter.GET("/", userCtrl.Index)
		userRouter.POST("/login", authCtrl.Login)
		userRouter.POST("/register", authCtrl.Register)
		userRouter.POST("/logout", authCtrl.Logout)
		userRouter.GET("/solves/:uuid", userSolveCtrl.Index)
		userRouter.GET("/solve/", userSolveCtrl.Show)
		userRouter.GET("/:uuid", userCtrl.Show)
		userRouter.PUT("/:uuid", userCtrl.Update)
	}

	apiRouter := r.Group("/api")
	{
		apiRouter.GET("/problems/:id/submissions", submissionCtrl.ListProblemSubmissions)
		apiRouter.GET("/users/:user_id/submissions", submissionCtrl.ListUserSubmissions)
	}

	learningRouter := r.Group("/learning")
	{
		learningRouter.GET("/users/:uuid/questions/:id/recommendations", learningCtrl.RecommendByQuestion)
	}

	questionRouter := r.Group("/question")
	{
		questionCtrl := adminquestion.NewQuestionController(models.DB, edgeBuilder)
		questionRouter.GET("/", questionCtrl.Index)
		questionRouter.GET("/new", questionCtrl.GetNewProblems)
		questionRouter.GET("/:id", questionCtrl.Show)
		questionRouter.POST("/", questionCtrl.Store)
		questionRouter.POST("/:id", questionCtrl.Update)
		questionRouter.DELETE("/delete", questionCtrl.DeleteProblem)
	}

	submissionRouter := r.Group("/submission")
	{
		submissionRouter.POST("/", submissionCtrl.SubmitCode)
		submissionRouter.GET("/:id", submissionCtrl.GetSubmissionResult)
	}

	testCaseRouter := r.Group("/testcase")
	{
		testCaseCtrl := admintestcase.NewTestCaseController(models.DB, ossClient)
		testCaseRouter.GET("/", testCaseCtrl.Index)
		testCaseRouter.GET("/question/:id", testCaseCtrl.GetByQuestion)
		testCaseRouter.GET("/:id", testCaseCtrl.Show)
		testCaseRouter.POST("/", testCaseCtrl.Store)
		testCaseRouter.POST("/batch", testCaseCtrl.BatchStore)
		testCaseRouter.POST("/oss/commit", testCaseCtrl.OSSCommit)
		testCaseRouter.PUT("/:id", testCaseCtrl.Update)
		testCaseRouter.DELETE("/:id", testCaseCtrl.Delete)
	}

	ojOverViewRouter := r.Group("/overview")
	{
		ojOverViewCtrl := controllers.NewOjOverViewController(models.DB)
		ojOverViewRouter.GET("/getHomeText", ojOverViewCtrl.GetHomeText)
		ojOverViewRouter.POST("/updateHomeText", ojOverViewCtrl.UpdateHomeText)
		ojOverViewRouter.GET("/getAnnouncement", ojOverViewCtrl.GetAnnouncement)
		ojOverViewRouter.POST("/updateAnnouncement", ojOverViewCtrl.UpdateAnnouncement)
	}

	if ossClient != nil {
		ossRouter := r.Group("/oss")
		{
			ossCtrl := adminstorage.NewOSSController(ossClient)
			ossRouter.POST("/upload", ossCtrl.UploadFile)
			ossRouter.GET("/upload-url", ossCtrl.GetUploadURL)
			ossRouter.GET("/files", ossCtrl.ListFiles)
		}
	}
}
