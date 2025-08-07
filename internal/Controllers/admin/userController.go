package admin

import (
    "crypto/md5"
    "encoding/hex"
    "fmt"
    "github.com/gin-gonic/gin"
    "dachuang/internal/config"
    "dachuang/internal/models"
    "dachuang/internal/services"
    "github.com/google/uuid"
    "gorm.io/gorm"
    "log"
    "net/http"
    
)

// MD5加密函数
func md5Encode(str string) string {
    h := md5.New()
    h.Write([]byte(str))
    return hex.EncodeToString(h.Sum(nil))
}

// LoginRequest 定义登录请求结构体
type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

// RegisterRequest 定义注册请求结构体
type RegisterRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

type LogoutRequest struct {
    UserID int `json:"user_id" binding:"required"`
}

type SubmitRequest struct {
    UserID         string `json:"user_id" binding:"required"`
    QuestionNumber int    `json:"question_number" binding:"required"`  // 改为题目编号
    Code           string `json:"code" binding:"required"`
}

type UserController struct {
    db             *gorm.DB
    judgeService   *services.JudgeService
    submissionQueue chan *models.Submission
}



// 初始化控制器
func NewUserController(db *gorm.DB, judgeAPI string) *UserController {
    queueSize := config.GlobalConfig.Judge.QueueSize
    controller := &UserController{
        db:             db,
        judgeService:   services.NewJudgeService(judgeAPI, db),
        submissionQueue: make(chan *models.Submission, queueSize),
    }
    
    // 启动消费者协程
    go controller.consumeSubmissions()
    
    return controller
}

// 消费者函数，处理消息队列中的提交信息
func (uc *UserController) consumeSubmissions() {
    for submission := range uc.submissionQueue {
        // 1. 更新提交状态为处理中
        submission.Status = "processing"
        if err := uc.db.Save(submission).Error; err != nil {
            log.Printf("保存提交状态失败 - 提交ID: %s, 错误: %v", submission.ID, err)
            continue
        }
        
        // 2. 调用评测服务
        if err := uc.judgeService.JudgeCode(submission); err != nil {
            log.Printf("评测失败 - 提交ID: %s, 错误: %v", submission.ID, err)
            submission.Status = "error"
            submission.Results = []models.TestCaseResult{
                {
                    Input:        "",
                    ActualOutput: fmt.Sprintf("评测系统错误: %v", err),
                    IsCorrect:    false,
                },
            }
        }
        
        // 3. 保存评测结果
        if err := uc.db.Save(submission).Error; err != nil {
            log.Printf("保存评测结果失败 - 提交ID: %s, 错误: %v", submission.ID, err)
        }
    }
}

func (uc *UserController) Index(c *gin.Context) {
    userList := []models.User{}
    uc.db.Find(&userList)
    c.JSON(200, gin.H{
        "result": userList,
    })
}

func (uc *UserController) Login(c *gin.Context) {
    var loginReq LoginRequest
    if err := c.ShouldBindJSON(&loginReq); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    encryptedPassword := md5Encode(loginReq.Password)

    var user models.User
    result := uc.db.Where("username = ? AND password = ?", loginReq.Username, encryptedPassword).First(&user)
    if result.Error != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":  "登录成功",
        "user_id":  user.Id,
        "uuid":     user.UUID,
        "username": user.Username,
    })
}

func (uc *UserController) Register(c *gin.Context) {
    var registerReq RegisterRequest
    if err := c.ShouldBindJSON(&registerReq); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var existingUser models.User
    result := uc.db.Where("username = ?", registerReq.Username).First(&existingUser)
    if result.Error == nil {
        c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
        return
    }

    var userUUID string
    var uuidExists bool
    for {
        userUUID = uuid.New().String()
        result := uc.db.Where("uuid = ?", userUUID).First(&existingUser)
        if result.Error != nil {
            uuidExists = false
            break
        }
        uuidExists = true
    }

    if uuidExists {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "无法生成唯一UUID"})
        return
    }

    encryptedPassword := md5Encode(registerReq.Password)

    newUser := models.User{
        Username: registerReq.Username,
        Password: encryptedPassword,
        UUID:     userUUID,
    }

    if err := uc.db.Create(&newUser).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "注册成功",
        "uuid":    userUUID,
        "user_id": newUser.Id,
    })
}

func (uc *UserController) Logout(c *gin.Context) {
    var logoutReq LogoutRequest
    if err := c.ShouldBindJSON(&logoutReq); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var user models.User
    result := uc.db.First(&user, logoutReq.UserID)
    if result.Error != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
        return
    }

    user.UUID = ""
    uc.db.Save(&user)

    c.JSON(http.StatusOK, gin.H{"message": "注销成功"})
}

func (uc *UserController) SubmitCode(c *gin.Context) {
    var submitRequest SubmitRequest

    if err := c.ShouldBindJSON(&submitRequest); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 验证用户是否存在
    var user models.User
    if err := uc.db.Where("uuid = ?", submitRequest.UserID).First(&user).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
        return
    }

    // 通过题目编号查找题目
    var question models.Question
    if err := uc.db.Where("question_number = ?", submitRequest.QuestionNumber).First(&question).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
        return
    }

    // 创建提交记录，使用题目的数据库ID
    submission := models.NewSubmission(submitRequest.UserID, question.Id, submitRequest.Code)

    if err := uc.db.Create(submission).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "提交创建失败"})
        return
    }

    // 将提交加入评测队列
    uc.submissionQueue <- submission

    c.JSON(http.StatusOK, gin.H{
        "submission_id":   submission.ID,
        "user_id":        submission.UserID,
        "question_number": submitRequest.QuestionNumber,  // 返回题目编号
        "question_id":    submission.QuestionID,         // 返回内部ID
        "status":         submission.Status,
        "message":        "代码已提交，正在评测中",
        "created_at":     submission.CreatedAt,
    })
}

func (uc *UserController) GetSubmissionResult(c *gin.Context) {
    submissionID := c.Param("id")
    
    var submission models.Submission
    if err := uc.db.Where("id = ?", submissionID).First(&submission).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "提交记录不存在"})
        return
    }
    
    passCount := 0
    for _, result := range submission.Results {
        if result.IsCorrect {
            passCount++
        }
    }
    passRate := float64(passCount) / float64(len(submission.Results))
    
    c.JSON(http.StatusOK, gin.H{
        "submission_id": submission.ID,
        "user_id":      submission.UserID,
        "question_id":  submission.QuestionID,
        "status":      submission.Status,
        "results":     submission.Results,
        "pass_rate":   passRate,
        "created_at": submission.CreatedAt,
        "updated_at": submission.UpdatedAt,
    })
}