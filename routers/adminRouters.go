package routers

import (
	"github.com/gin-gonic/gin"
	"dachuang/Controllers/admin"
	"dachuang/models"
)

func AdminRoutersInit(r *gin.Engine,) {
	adminRouter := r.Group("/admin") 
	{
	

	//创建user实例
	//userCtrl:=&admin.UserController{}
	 userCtrl := admin.NewUserController(models.DB, "http://your-judge-service-api")

	adminRouter.GET("/user",userCtrl.Index)
	adminRouter.POST("/login", userCtrl.Login) // 新增登录路由
	adminRouter.POST("/register", userCtrl.Register) // 新增注册路由
	adminRouter.POST("/logout", userCtrl.Logout) // 新增注销路由
	adminRouter.POST("/submit", userCtrl.SubmitCode) // 新增提交代码路由
    adminRouter.GET("/submission/:id", userCtrl.GetSubmissionResult)
	//创建question实例
	questionCtrl:=&admin.QuestionController{}
	adminRouter.GET("/question",questionCtrl.Index)
	adminRouter.POST("/question", questionCtrl.Store)
	adminRouter.POST("/question/:id", questionCtrl.Update)
	//创建categories实例
	categoryCtrl:=&admin.CategoryController{}
	adminRouter.GET("/category",categoryCtrl.Index)
	adminRouter.POST("/category", categoryCtrl.Store)
	adminRouter.POST("/category/:id", categoryCtrl.Update)
	// 关系相关路由
        relationCtrl := &admin.RelationController{}
        adminRouter.GET("/relations", relationCtrl.Index)
	// 节点相关路由
        nodeCtrl := &admin.NodeController{}
        adminRouter.GET("/nodes", nodeCtrl.Index)
	
	}
	

}	
