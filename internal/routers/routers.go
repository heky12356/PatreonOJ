package routers

import (
	"context"
	"net/http"
	controllers "patreonoj/internal/Controllers"
	adminauth "patreonoj/internal/Controllers/auth"
	debugctrl "patreonoj/internal/Controllers/debug"
	adminlearning "patreonoj/internal/Controllers/learning"
	adminquestion "patreonoj/internal/Controllers/question"
	adminsite "patreonoj/internal/Controllers/site"
	adminstorage "patreonoj/internal/Controllers/storage"
	adminsubmission "patreonoj/internal/Controllers/submission"
	admintestcase "patreonoj/internal/Controllers/testcase"
	adminuser "patreonoj/internal/Controllers/user"
	"patreonoj/internal/config"
	"patreonoj/internal/middleware"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	debugsvc "patreonoj/internal/services/debug"
	"patreonoj/internal/services/judge"
	questionsvc "patreonoj/internal/services/question"
	"time"

	"github.com/gin-gonic/gin"
)

func RoutersInit(r *gin.Engine, ossClient *oss.OSS) func(context.Context) error {
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
	var debugRunner debugsvc.Runner
	if config.GlobalConfig.Judge.GoJudge.Enabled {
		client := judge.NewGoJudgeClient(config.GlobalConfig.Judge.GoJudge.APIURL, 120)
		client.Token = config.GlobalConfig.Judge.GoJudge.Token
		debugRunner = client
	}
	var sampleObjects debugsvc.ObjectStore
	if ossClient != nil {
		sampleObjects = ossClient
	}
	debugService := debugsvc.NewService(models.DB, debugRunner, sampleObjects, config.GlobalConfig.OSS.BucketName)
	debugController := &debugctrl.Controller{Service: debugService}
	r.GET("/question/:id/samples", debugController.Samples)
	debugRouter := r.Group("/debug/runs").Use(middleware.RequireAuth())
	debugRouter.POST("", debugController.Create)
	debugRouter.GET("/:id", debugController.Get)
	debugRouter.DELETE("/:id", debugController.Cancel)
	siteCtrl := adminsite.NewController(models.DB, ossClient)
	r.GET("/site/settings", siteCtrl.Get)
	r.GET("/site/images/:name", siteCtrl.Image)
	r.PUT("/site/settings", middleware.RequirePermission("admin"), siteCtrl.Update)
	r.POST("/site/images", middleware.RequirePermission("admin"), siteCtrl.UploadImage)
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		db, err := models.DB.DB()
		if err != nil || db.PingContext(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authUserRouter := r.Group("/user").Use(middleware.RequireAuth())
	{
		authUserRouter.GET("/", userCtrl.Index)
		authUserRouter.GET("/me", userCtrl.GetProfile)
		authUserRouter.GET("/roles", userCtrl.Roles)
		authUserRouter.GET("/me/problem-status", userCtrl.ProblemStatuses)
		authUserRouter.GET("/solve", userSolveCtrl.Show)
		authUserRouter.POST("/logout", authCtrl.Logout)
		authUserRouter.GET("/solves/:uuid", userSolveCtrl.Index)
		authUserRouter.GET("/:id", userCtrl.Show)
		authUserRouter.PUT("/:id", userCtrl.Update)
		authUserRouter.PUT("/update/:id", userCtrl.Update)
	}

	userRouter := r.Group("/user")
	{
		throttle := middleware.AuthThrottle(config.GlobalConfig.Server.AuthMaxConcurrent, config.GlobalConfig.Server.AuthRequestsPerMinute)
		userRouter.POST("/login", throttle, authCtrl.Login)
		userRouter.POST("/register", throttle, authCtrl.Register)
		// 获取用户详细
		userRouter.GET("/:id/public", userCtrl.GetProfilePublic)
		userRouter.GET("/:id/home", userCtrl.Home)
	}

	// 查询提交记录
	apiRouter := r.Group("/api")
	{
		apiRouter.GET("/problems/:id/submissions", submissionCtrl.ListProblemSubmissions)
		apiRouter.GET("/users/:user_id/submissions", middleware.RequireAuth(), submissionCtrl.ListUserSubmissions)
	}

	learningRouter := r.Group("/learning")
	{
		learningRouter.GET("/tags", learningCtrl.Tags)
		learningRouter.GET("/graph", learningCtrl.TagGraph)
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
	return func(ctx context.Context) error {
		debugErr := debugService.Shutdown(ctx)
		if err := submissionCtrl.Shutdown(ctx); err != nil {
			return err
		}
		return debugErr
	}
}
