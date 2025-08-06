package main

import (
    "html/template" 
    "time"
    "dachuang/routers"
    "github.com/gin-gonic/gin"
    
)

// UserInfo 结构体定义（当前未使用）
type UserInfo struct {
    Username string `json:"username" form:"username"`
    Password string `json:"password" form:"password"`
}

// UnixToTime 将Unix时间戳转换为格式化的时间字符串
func UnixToTime(timestamp int64) string {
    return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}

func main() {
    // 创建一个默认的路由引擎
    r := gin.Default()

    // 自定义模板函数 - 注意要放在加载模板前面
    r.SetFuncMap(template.FuncMap{ // 修正：从templates.FuncMap改为template.FuncMap
        "UnixToTime": UnixToTime,
    })

    // 加载模板 - 放在配置路由前面
    r.LoadHTMLGlob("templates/**/*")

    // 配置静态web目录 - 第一个参数表示路由，第二个参数表示映射的目录
    r.Static("/static", "./static")
   
   


    routers.AdminRoutersInit(r)

    
    r.Run(":8080") 
}