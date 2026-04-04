package admin

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/graph"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	"patreonoj/internal/services"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubmissionController 处理代码提交相关的请求
type SubmissionController struct {
	db              *gorm.DB
	judgeService    *services.JudgeService
	queryService    *services.SubmissionQueryService
	commandService  *services.SubmissionCommandService
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
	judgeService := services.NewJudgeService(&config.GlobalConfig.Judge, db, ossClient, bucket, graphService, assessmentService)
	controller := &SubmissionController{
		db:              db,
		judgeService:    judgeService,
		queryService:    services.NewSubmissionQueryService(db),
		commandService:  services.NewSubmissionCommandService(db, judgeService),
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
			submission.ErrorCode = services.EnsureSubmissionErrorCode(err)
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

		parsedResults := services.ParseSubmissionResults(submission.Results)
		for _, result := range parsedResults {
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

		if !allcurrent || submission.Status != "completed" || len(parsedResults) == 0 {
			continue
		}

		// 4. 更新用户解题列表
		if question.Id == 0 {
			_ = sc.db.Where("id = ?", submission.QuestionID).First(&question)
		}

		// 写入图谱 SOLVED 边
		if sc.graphService != nil {
			if err := sc.graphService.MarkUserSolvedQuestion(context.Background(), submission.UserID, question.QuestionNumber); err != nil {
				log.Printf("写入SOLVED边失败 user=%s question=%d err=%v", submission.UserID, question.QuestionNumber, err)
			}
		}

		// 更新用户解题记录
		rec := models.UserSolvedQuestion{
			UserUUID:   submission.UserID,
			QuestionID: submission.QuestionID,
			SolvedAt:   time.Now(),
		}

		if err := sc.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_uuid"}, {Name: "question_id"}},
			DoNothing: true, // 或 DoUpdates 更新 solved_at
		}).Create(&rec).Error; err != nil {
			log.Printf("创建用户解题记录失败 - 用户ID: %s, 题目ID: %d, 错误: %v", rec.UserUUID, rec.QuestionID, err)
		}
	}
}

// SubmitCode 提交代码
func (sc *SubmissionController) SubmitCode(c *gin.Context) {
	var submitRequest SubmitRequest

	if err := c.ShouldBindJSON(&submitRequest); err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := sc.commandService.CreateSubmission(submitRequest.UserID, submitRequest.QuestionNumber, submitRequest.Code)
	if errors.Is(err, services.ErrSubmissionUserNotFound) {
		apiError(c, http.StatusNotFound, "用户不存在")
		return
	}
	if errors.Is(err, services.ErrSubmissionUserDisabled) {
		apiError(c, http.StatusForbidden, "用户已被禁用")
		return
	}
	if errors.Is(err, services.ErrSubmissionQuestionNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "提交创建失败")
		return
	}

	submission := result.Submission

	// 将提交加入评测队列
	sc.submissionQueue <- submission

	apiOK(c, http.StatusOK, "代码已提交，正在评测中", gin.H{
		"submission_id":   submission.ID,
		"user_id":         submission.UserID,
		"question_number": result.QuestionNumber, // 返回题目编号
		"question_id":     submission.QuestionID, // 返回内部ID
		"status":          submission.Status,
		"message":         "代码已提交，正在评测中",
		"created_at":      submission.CreatedAt,
	})
}

// GetSubmissionResult 获取代码提交的评测结果
func (sc *SubmissionController) GetSubmissionResult(c *gin.Context) {
	submissionID := c.Param("id")
	response, err := sc.commandService.BuildSubmissionResult(submissionID)
	if errors.Is(err, services.ErrSubmissionNotFound) {
		apiError(c, http.StatusNotFound, "提交记录不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询失败")
		return
	}

	msg := "ok"
	if m, ok := response["message"].(string); ok && m != "" {
		msg = m
	}
	apiOK(c, http.StatusOK, msg, response)
}

// ListProblemSubmissions 获取题目提交记录（公开）
func (sc *SubmissionController) ListProblemSubmissions(c *gin.Context) {
	qn, err := strconv.Atoi(strings.TrimSpace(c.Param("question_number")))
	if err != nil || qn <= 0 {
		apiError(c, http.StatusBadRequest, "question_number 无效")
		return
	}

	page, size, err := parsePageSizeQuery(c, "page", "size", 1, 20, 100)
	if err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	language := strings.TrimSpace(c.Query("language"))
	result, err := sc.queryService.ListProblemSubmissions(qn, status, language, page, size)
	if errors.Is(err, services.ErrSubmissionQuestionNotFound) {
		apiError(c, http.StatusNotFound, "题目不存在")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	apiOK(c, http.StatusOK, "ok", gin.H{
		"total": result.Total,
		"page":  result.Page,
		"size":  result.Size,
		"pages": result.Pages,
		"items": result.Items,
	})
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
		apiError(c, http.StatusBadRequest, "user_id 无效")
		return
	}
	if opUUID != targetUUID && !isAdmin {
		apiError(c, http.StatusForbidden, "无权限")
		return
	}

	page, size, err := parsePageSizeQuery(c, "page", "size", 1, 20, 100)
	if err != nil {
		apiError(c, http.StatusBadRequest, err.Error())
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	language := strings.TrimSpace(c.Query("language"))
	problemID := strings.TrimSpace(c.Query("problem_id"))
	result, err := sc.queryService.ListUserSubmissions(targetUUID, problemID, status, language, page, size)
	if errors.Is(err, services.ErrSubmissionProblemIDInvalid) {
		apiError(c, http.StatusBadRequest, "problem_id 无效")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "查询失败")
		return
	}
	apiOK(c, http.StatusOK, "ok", gin.H{
		"total": result.Total,
		"page":  result.Page,
		"size":  result.Size,
		"pages": result.Pages,
		"items": result.Items,
	})
}
