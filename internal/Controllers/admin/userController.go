package admin

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"dachuang/internal/config"
	"dachuang/internal/models"
	"dachuang/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MD5加密函数
// getErrorCode 根据错误信息返回对应的错误码
func getErrorCode(err error) string {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "测试用例"):
		return "E001" // 测试用例相关错误
	case strings.Contains(errMsg, "编译"):
		return "E002" // 编译错误
	case strings.Contains(errMsg, "超时"):
		return "E003" // 运行超时
	case strings.Contains(errMsg, "内存"):
		return "E004" // 内存超限
	case strings.Contains(errMsg, "网络"):
		return "E005" // 网络错误
	default:
		return "E999" // 未知错误
	}
}

// parseResults 解析Results字符串为TestCaseResult数组
func parseResults(resultsStr string) []models.TestCaseResult {
	if resultsStr == "" {
		return []models.TestCaseResult{}
	}

	var results []models.TestCaseResult
	if err := json.Unmarshal([]byte(resultsStr), &results); err != nil {
		log.Printf("解析评测结果失败: %v", err)
		return []models.TestCaseResult{}
	}
	return results
}

// serializeResults 将TestCaseResult数组序列化为JSON字符串
func serializeResults(results []models.TestCaseResult) string {
	if len(results) == 0 {
		return ""
	}

	data, err := json.Marshal(results)
	if err != nil {
		log.Printf("序列化评测结果失败: %v", err)
		return ""
	}
	return string(data)
}

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
	QuestionNumber int    `json:"question_number" binding:"required"` // 改为题目编号
	Code           string `json:"code" binding:"required"`
}

type UserController struct {
	db              *gorm.DB
	judgeService    *services.JudgeService
	submissionQueue chan *models.Submission
}

// 初始化控制器
func NewUserController(db *gorm.DB) *UserController {
	queueSize := config.GlobalConfig.Judge.QueueSize
	controller := &UserController{
		db:              db,
		judgeService:    services.NewJudgeService(&config.GlobalConfig.Judge, db),
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

			// 设置错误码和错误信息
			submission.ErrorCode = getErrorCode(err)
			submission.ErrorMsg = err.Error()
			submission.Results = "" // 清空结果
		}

		// 3. 保存评测结果
		if err := uc.db.Save(submission).Error; err != nil {
			log.Printf("保存评测结果失败 - 提交ID: %s, 错误: %v", submission.ID, err)
		}

		allcurrent := true
		log.Printf("评测结果: %s", submission.Results)

		for _, result := range parseResults(submission.Results) {
			if !result.IsCorrect {
				allcurrent = false
				break
			}
		}
		log.Printf("allcurrent: %v", allcurrent)

		if !allcurrent {
			continue
		}

		// 4. 更新用户解题列表
		var usersolve models.UserSolve
		if err := uc.db.Where("uuid = ?", submission.UserID).First(&usersolve).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				usersolve.UUID = submission.UserID
				usersolve.ProblemIDs = strconv.Itoa(submission.QuestionID)
				uc.db.Create(&usersolve)
			} else {
				log.Printf("查询用户失败 - 用户ID: %s, 错误: %v", submission.UserID, err)
				continue
			}
		}

		var question models.Question

		_ = uc.db.Where("id = ?", submission.QuestionID).First(&question)

		qid := strconv.Itoa(question.QuestionNumber)
		log.Printf("usersolve.ProblemIDs: %s", usersolve.ProblemIDs)

		if strings.Contains(usersolve.ProblemIDs, qid) {
			log.Printf("BUG: %s", qid)
			continue
		}

		log.Printf("qid: %s", qid)

		if usersolve.ProblemIDs == "" {
			usersolve.ProblemIDs = qid
		} else {
			usersolve.ProblemIDs += "," + qid
		}

		// 更新用户解题记录
		if err := uc.db.Model(&usersolve).Update("problem_ids", usersolve.ProblemIDs).Error; err != nil {
			log.Printf("更新用户解题记录失败 - 用户ID: %s, 错误: %v", usersolve.UUID, err)
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
		"user_id":         submission.UserID,
		"question_number": submitRequest.QuestionNumber, // 返回题目编号
		"question_id":     submission.QuestionID,        // 返回内部ID
		"status":          submission.Status,
		"message":         "代码已提交，正在评测中",
		"created_at":      submission.CreatedAt,
	})
}

// GetSubmissionResult 获取代码提交的评测结果
func (uc *UserController) GetSubmissionResult(c *gin.Context) {
	submissionID := c.Param("id")

	// 查询提交记录
	var submission models.Submission
	if err := uc.db.Where("id = ?", submissionID).First(&submission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "提交记录不存在"})
		return
	}

	// 构建基础响应数据
	response := gin.H{
		"submission_id": submission.ID,
		"user_id":       submission.UserID,
		"question_id":   submission.QuestionID,
		"status":        submission.Status,
		"created_at":    submission.CreatedAt,
		"updated_at":    submission.UpdatedAt,
	}

	// 查询题目信息以获取题目编号
	var question models.Question
	if err := uc.db.Where("id = ?", submission.QuestionID).First(&question).Error; err == nil {
		response["question_number"] = question.QuestionNumber
	}

	// 根据评测状态返回不同的数据
	switch submission.Status {
	case "completed":
		// 评测完成：解析结果并计算通过率
		results := parseResults(submission.Results)
		if len(results) > 0 {
			passCount := 0
			for _, result := range results {
				if result.IsCorrect {
					passCount++
				}
			}
			response["results"] = results
			response["pass_rate"] = float64(passCount) / float64(len(results))
			response["total_cases"] = len(results)
			response["passed_cases"] = passCount
		} else {
			// 评测完成但无结果（异常情况）
			response["results"] = []models.TestCaseResult{}
			response["pass_rate"] = 0.0
			response["total_cases"] = 0
			response["passed_cases"] = 0
			response["message"] = "评测完成但无测试结果"
		}
	case "processing":
		// 评测中：返回处理状态
		response["results"] = []models.TestCaseResult{}
		response["message"] = "代码正在评测中，请稍后查询"
	case "error":
		// 评测出错：返回错误信息和错误码
		response["results"] = []models.TestCaseResult{}
		response["error_code"] = submission.ErrorCode
		response["error_message"] = submission.ErrorMsg
		response["message"] = getErrorMessage(submission.ErrorCode)
	default:
		// 等待评测：返回等待状态
		response["results"] = []models.TestCaseResult{}
		response["message"] = "代码已提交，等待评测"
	}

	c.JSON(http.StatusOK, response)
}

// getErrorMessage 根据错误码返回用户友好的错误信息
func getErrorMessage(errorCode string) string {
	switch errorCode {
	case "E001":
		return "题目配置错误：缺少测试用例，请联系管理员"
	case "E002":
		return "代码编译失败，请检查语法错误"
	case "E003":
		return "代码运行超时，请优化算法效率"
	case "E004":
		return "内存使用超限，请优化内存使用"
	case "E005":
		return "网络连接错误，请稍后重试"
	case "E999":
		return "系统内部错误，请联系管理员"
	default:
		return "未知错误，请联系管理员"
	}
}
