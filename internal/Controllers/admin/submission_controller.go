package admin

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/graph"
	"dachuang/internal/models"
	"dachuang/internal/oss"
	"dachuang/internal/services"
	"dachuang/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SubmissionController 处理代码提交相关的请求
type SubmissionController struct {
	db              *gorm.DB
	judgeService    *services.JudgeService
	graphService    *graph.QuestionGraphService
	submissionQueue chan *models.Submission
}

// NewSubmissionController 创建提交控制器
func NewSubmissionController(db *gorm.DB, ossClient *oss.OSS, graphService *graph.QuestionGraphService) *SubmissionController {
	queueSize := config.GlobalConfig.Judge.QueueSize
	bucket := config.GlobalConfig.OSS.BucketName
	assessmentService := services.NewAssessmentService(db, graphService)
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}
	controller := &SubmissionController{
		db:              db,
		judgeService:    services.NewJudgeService(&config.GlobalConfig.Judge, db, ossClient, bucket, graphService, assessmentService),
		graphService:    graphService,
		submissionQueue: make(chan *models.Submission, queueSize),
	}

	// 启动消费者协程
	go controller.consumeSubmissions()

	return controller
}

// SubmitRequest 提交代码请求结构体
type SubmitRequest struct {
	UserID         string `json:"user_id" binding:"required"`
	QuestionNumber int    `json:"question_number" binding:"required"`
	Code           string `json:"code" binding:"required"`
}

// submissionListItem 提交记录列表项
type submissionListItem struct {
	SubmissionID   string    `json:"submission_id"`
	UserID         string    `json:"user_id"`
	QuestionNumber int       `json:"question_number"`
	SubmittedAt    time.Time `json:"submitted_at"`
	Status         string    `json:"status"`
	RuntimeMs      int64     `json:"runtime_ms"`
	MemoryKB       int64     `json:"memory_kb"`
	Language       string    `json:"language"`
	CodeLength     int       `json:"code_length"`
}

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

// consumeSubmissions 消费者函数，处理消息队列中的提交信息
func (sc *SubmissionController) consumeSubmissions() {
	for submission := range sc.submissionQueue {
		// 1. 更新提交状态为处理中
		submission.Status = "processing"
		if err := sc.db.Save(submission).Error; err != nil {
			log.Printf("保存提交状态失败 - 提交ID: %s, 错误: %v", submission.ID, err)
			continue
		}

		// 2. 调用评测服务
		if err := sc.judgeService.JudgeCode(submission); err != nil {
			log.Printf("评测失败 - 提交ID: %s, 错误: %v", submission.ID, err)
			submission.Status = "error"

			// 设置错误码和错误信息
			submission.ErrorCode = getErrorCode(err)
			submission.ErrorMsg = err.Error()
			submission.Results = "" // 清空结果
		}

		// 3. 保存评测结果
		if err := sc.db.Save(submission).Error; err != nil {
			log.Printf("保存评测结果失败 - 提交ID: %s, 错误: %v", submission.ID, err)
		}

		// 判断是否AC
		allcurrent := true
		log.Printf("评测结果: %s", submission.Results)

		for _, result := range parseResults(submission.Results) {
			if !result.IsCorrect {
				allcurrent = false
				break
			}
		}
		log.Printf("allcurrent: %v", allcurrent)

		var question models.Question
		if err := sc.db.Where("id = ?", submission.QuestionID).First(&question).Error; err != nil {
			log.Printf("查询题目失败 - 题目ID: %d, 错误: %v", submission.QuestionID, err)
			continue
		}

		if !allcurrent || submission.Status != "completed" || len(parseResults(submission.Results)) == 0 {
			continue
		}

		// 4. 更新用户解题列表
		if question.Id == 0 {
			_ = sc.db.Where("id = ?", submission.QuestionID).First(&question)
		}

		var usersolve models.UserSolve
		if err := sc.db.Where("uuid = ?", submission.UserID).First(&usersolve).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				usersolve.UUID = submission.UserID
				usersolve.ProblemIDs = strconv.Itoa(question.QuestionNumber)
				if err := sc.db.Create(&usersolve).Error; err != nil {
					log.Printf("创建用户解题记录失败 - 用户ID: %s, 错误: %v", submission.UserID, err)
					continue
				}
			} else {
				log.Printf("查询用户失败 - 用户ID: %s, 错误: %v", submission.UserID, err)
				continue
			}
		}

		qid := strconv.Itoa(question.QuestionNumber)
		log.Printf("usersolve.ProblemIDs: %s", usersolve.ProblemIDs)

		// 写入图谱 SOLVED 边
		if sc.graphService != nil {
			if err := sc.graphService.MarkUserSolvedQuestion(context.Background(), submission.UserID, question.QuestionNumber); err != nil {
				log.Printf("写入SOLVED边失败 user=%s question=%d err=%v", submission.UserID, question.QuestionNumber, err)
			}
		}

		// 去重
		acSet := strings.Split(usersolve.ProblemIDs, ",")
		if slices.Contains(acSet, qid) {
			log.Printf("用户已解题: %s", qid)
			continue
		}

		log.Printf("qid: %s", qid)

		if usersolve.ProblemIDs == "" {
			usersolve.ProblemIDs = qid
		} else {
			usersolve.ProblemIDs += "," + qid
		}

		// 更新用户解题记录
		if err := sc.db.Model(&usersolve).Update("problem_ids", usersolve.ProblemIDs).Error; err != nil {
			log.Printf("更新用户解题记录失败 - 用户ID: %s, 错误: %v", usersolve.UUID, err)
		}
	}
}

// SubmitCode 提交代码
func (sc *SubmissionController) SubmitCode(c *gin.Context) {
	var submitRequest SubmitRequest

	if err := c.ShouldBindJSON(&submitRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证用户是否存在
	var user models.User
	if err := sc.db.Where("uuid = ?", submitRequest.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if user.Status != "" && user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return
	}

	// 通过题目编号查找题目
	var question models.Question
	if err := sc.db.Where("question_number = ?", submitRequest.QuestionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	lang := ""
	if sc.judgeService != nil {
		lang = sc.judgeService.DetectLanguage(submitRequest.Code)
	}

	// 创建提交记录，使用题目的数据库ID
	submission := models.NewSubmission(submitRequest.UserID, question.Id, submitRequest.Code, lang)

	if err := sc.db.Create(submission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交创建失败"})
		return
	}

	// 将提交加入评测队列
	sc.submissionQueue <- submission

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
func (sc *SubmissionController) GetSubmissionResult(c *gin.Context) {
	submissionID := c.Param("id")

	// 查询提交记录
	var submission models.Submission
	if err := sc.db.Where("id = ?", submissionID).First(&submission).Error; err != nil {
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
	if err := sc.db.Where("id = ?", submission.QuestionID).First(&question).Error; err == nil {
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

// ListProblemSubmissions 获取题目提交记录（公开）
func (sc *SubmissionController) ListProblemSubmissions(c *gin.Context) {
	qn, err := strconv.Atoi(strings.TrimSpace(c.Param("question_number")))
	if err != nil || qn <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question_number 无效"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	var question models.Question
	if err := sc.db.Where("question_number = ?", qn).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	language := strings.TrimSpace(c.Query("language"))

	countQ := sc.db.Table("submissions").Joins("JOIN question ON question.id = submissions.question_id").Where("submissions.question_id = ?", question.Id)
	if status != "" {
		countQ = countQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		countQ = countQ.Where("submissions.language = ?", language)
	}

	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	items := make([]submissionListItem, 0, size)
	listQ := sc.db.Table("submissions").
		Select("submissions.id AS submission_id, submissions.user_id, question.question_number, submissions.created_at AS submitted_at, submissions.status, submissions.runtime_ms, submissions.memory_kb, submissions.language, submissions.code_length").
		Joins("JOIN question ON question.id = submissions.question_id").
		Where("submissions.question_id = ?", question.Id)
	if status != "" {
		listQ = listQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		listQ = listQ.Where("submissions.language = ?", language)
	}
	if err := listQ.Order("submissions.created_at desc").Limit(size).Offset(size * (page - 1)).Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	pages := int64(0)
	if total > 0 {
		pages = int64(math.Ceil(float64(total) / float64(size)))
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "page": page, "size": size, "pages": pages, "items": items})
}

// ListUserSubmissions 获取用户提交记录
func (sc *SubmissionController) ListUserSubmissions(c *gin.Context) {
	opUUID, ok := requireOperatorUUID(sc.db, c)
	if !ok {
		return
	}
	isAdmin := util.UserInstance.HasPermission(opUUID, "admin")

	targetUUID := strings.TrimSpace(c.Param("user_id"))
	if targetUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id 无效"})
		return
	}
	if opUUID != targetUUID && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	status := strings.TrimSpace(c.Query("status"))
	language := strings.TrimSpace(c.Query("language"))

	var questionID *int
	if v := strings.TrimSpace(c.Query("problem_id")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			var q models.Question
			if err := sc.db.Where("question_number = ?", n).First(&q).Error; err == nil {
				questionID = &q.Id
			} else {
				questionID = &n
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "problem_id 无效"})
			return
		}
	}

	countQ := sc.db.Table("submissions").Joins("JOIN question ON question.id = submissions.question_id").Where("submissions.user_id = ?", targetUUID)
	if questionID != nil {
		countQ = countQ.Where("submissions.question_id = ?", *questionID)
	}
	if status != "" {
		countQ = countQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		countQ = countQ.Where("submissions.language = ?", language)
	}

	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	items := make([]submissionListItem, 0, size)
	listQ := sc.db.Table("submissions").
		Select("submissions.id AS submission_id, submissions.user_id, question.question_number, submissions.created_at AS submitted_at, submissions.status, submissions.runtime_ms, submissions.memory_kb, submissions.language, submissions.code_length").
		Joins("JOIN question ON question.id = submissions.question_id").
		Where("submissions.user_id = ?", targetUUID)
	if questionID != nil {
		listQ = listQ.Where("submissions.question_id = ?", *questionID)
	}
	if status != "" {
		listQ = listQ.Where("submissions.status = ?", status)
	}
	if language != "" {
		listQ = listQ.Where("submissions.language = ?", language)
	}
	if err := listQ.Order("submissions.created_at desc").Limit(size).Offset(size * (page - 1)).Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	pages := int64(0)
	if total > 0 {
		pages = int64(math.Ceil(float64(total) / float64(size)))
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "page": page, "size": size, "pages": pages, "items": items})
}
