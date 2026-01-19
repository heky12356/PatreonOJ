package admin

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
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
	db           *gorm.DB
	judgeService *services.JudgeService
	graphService *graph.QuestionGraphService

	submissionQueue chan *models.Submission
}

// 初始化控制器
func NewUserController(db *gorm.DB, ossClient *oss.OSS, graphService *graph.QuestionGraphService) *UserController {
	queueSize := config.GlobalConfig.Judge.QueueSize
	bucket := config.GlobalConfig.OSS.BucketName
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}
	controller := &UserController{
		db:              db,
		judgeService:    services.NewJudgeService(&config.GlobalConfig.Judge, db, ossClient, bucket),
		graphService:    graphService,
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

		var question models.Question
		if err := uc.db.Where("id = ?", submission.QuestionID).First(&question).Error; err == nil {
			_ = uc.recordMastery(submission.UserID, question, allcurrent, time.Now())
		}

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

		if question.Id == 0 {
			_ = uc.db.Where("id = ?", submission.QuestionID).First(&question)
		}

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

// UpdateUserRequest 更新用户请求结构体
type UpdateUserRequest struct {
	Nickname    *string  `json:"nickname"`
	Email       *string  `json:"email"`
	AvatarURL   *string  `json:"avatar_url"`
	Status      *string  `json:"status"`
	Permissions *string  `json:"permissions"`
	RoleCodes   []string `json:"role_codes"`
}

// Show 获取用户详情
func (uc *UserController) Show(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	var user models.User
	if err := uc.db.Preload("Roles.Permissions").Where("uuid = ?", targetUUID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": user})
}

// Update 更新用户信息
func (uc *UserController) Update(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isAdmin := util.UserInstance.HasPermission(operatorUUID, "admin")
	if !isAdmin {
		if req.Status != nil || req.Permissions != nil || len(req.RoleCodes) > 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员可修改权限/角色/状态"})
			return
		}
	}

	var user models.User
	if err := uc.db.Preload("Roles").Where("uuid = ?", targetUUID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	updates := map[string]any{}
	if req.Nickname != nil {
		updates["nickname"] = strings.TrimSpace(*req.Nickname)
	}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = strings.TrimSpace(*req.AvatarURL)
	}
	if req.Status != nil {
		s := strings.TrimSpace(*req.Status)
		if s != "active" && s != "disabled" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status 仅支持 active/disabled"})
			return
		}
		updates["status"] = s
	}
	if req.Permissions != nil {
		updates["permissions"] = strings.TrimSpace(*req.Permissions)
	}

	if len(updates) > 0 {
		if err := uc.db.Model(&user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
	}

	if len(req.RoleCodes) > 0 {
		roles := make([]models.Role, 0, len(req.RoleCodes))
		if err := uc.db.Where("code IN ?", req.RoleCodes).Find(&roles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询角色失败"})
			return
		}
		if len(roles) != len(req.RoleCodes) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role_codes 包含不存在的角色"})
			return
		}
		if err := uc.db.Model(&user).Association("Roles").Replace(&roles); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "绑定角色失败"})
			return
		}
	}

	_ = uc.db.Preload("Roles.Permissions").Where("uuid = ?", targetUUID).First(&user)
	c.JSON(http.StatusOK, gin.H{"result": user})
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

	if user.Status != "" && user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return
	}

	permissions := strings.Split(user.Permissions, ",")

	c.JSON(http.StatusOK, gin.H{
		"message":     "登录成功",
		"user_id":     user.Id,
		"uuid":        user.UUID,
		"username":    user.Username,
		"permissions": permissions,
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
	if user.Status != "" && user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return
	}

	// 通过题目编号查找题目
	var question models.Question
	if err := uc.db.Where("question_number = ?", submitRequest.QuestionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	lang := ""
	if uc.judgeService != nil {
		lang = uc.judgeService.DetectLanguage(submitRequest.Code)
	}

	// 创建提交记录，使用题目的数据库ID
	submission := models.NewSubmission(submitRequest.UserID, question.Id, submitRequest.Code, lang)

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

// SubmitMasteryEvent 提交用户对题目或标签的掌握度事件
type MasteryEventRequest struct {
	QuestionNumber int  `json:"question_number" binding:"required"`
	Accepted       bool `json:"accepted"`
}

// SubmitMasteryEvent 提交用户对题目或标签的掌握度事件
func (uc *UserController) SubmitMasteryEvent(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	var req MasteryEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var question models.Question
	if err := uc.db.Where("question_number = ?", req.QuestionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}
	if err := uc.recordMastery(targetUUID, question, req.Accepted, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入掌握度失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ListQuestionMastery 获取用户对所有题目掌握度
func (uc *UserController) ListQuestionMastery(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	pageIdx, _ := strconv.Atoi(c.DefaultQuery("pageIdx", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageIdx < 1 {
		pageIdx = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := uc.db.Model(&models.UserQuestionMastery{}).Where("user_uuid = ?", targetUUID)
	if v := c.Query("question_number"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			q = q.Where("question_number = ?", n)
		}
	}
	if v := c.Query("min_mastery"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			q = q.Where("mastery >= ?", f)
		}
	}
	if v := c.Query("max_mastery"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			q = q.Where("mastery <= ?", f)
		}
	}

	sortKey := c.DefaultQuery("sort", "updated_at")
	order := strings.ToLower(c.DefaultQuery("order", "desc"))
	if order != "asc" {
		order = "desc"
	}
	col := map[string]string{"question_number": "question_number", "attempts": "attempts", "accepted_count": "accepted_count", "mastery": "mastery", "created_at": "created_at", "updated_at": "updated_at"}[sortKey]
	if col == "" {
		col = "updated_at"
	}
	q = q.Order(col + " " + order)

	var totalCnt int64
	q.Count(&totalCnt)
	items := []models.UserQuestionMastery{}
	q.Limit(pageSize).Offset(pageSize * (pageIdx - 1)).Find(&items)
	c.JSON(http.StatusOK, gin.H{"result": items, "pageIdx": pageIdx, "pageSize": pageSize, "totalCnt": totalCnt})
}

// ListTagMastery 获取用户对所有标签掌握度
func (uc *UserController) ListTagMastery(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	pageIdx, _ := strconv.Atoi(c.DefaultQuery("pageIdx", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageIdx < 1 {
		pageIdx = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := uc.db.Model(&models.UserTagMastery{}).Where("user_uuid = ?", targetUUID)
	if v := strings.TrimSpace(c.Query("tag")); v != "" {
		q = q.Where("tag = ?", v)
	}
	if v := c.Query("min_mastery"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			q = q.Where("mastery >= ?", f)
		}
	}
	if v := c.Query("max_mastery"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			q = q.Where("mastery <= ?", f)
		}
	}

	sortKey := c.DefaultQuery("sort", "updated_at")
	order := strings.ToLower(c.DefaultQuery("order", "desc"))
	if order != "asc" {
		order = "desc"
	}
	col := map[string]string{"tag": "tag", "attempts": "attempts", "accepted_count": "accepted_count", "mastery": "mastery", "created_at": "created_at", "updated_at": "updated_at"}[sortKey]
	if col == "" {
		col = "updated_at"
	}
	q = q.Order(col + " " + order)

	var totalCnt int64
	q.Count(&totalCnt)
	items := []models.UserTagMastery{}
	q.Limit(pageSize).Offset(pageSize * (pageIdx - 1)).Find(&items)
	c.JSON(http.StatusOK, gin.H{"result": items, "pageIdx": pageIdx, "pageSize": pageSize, "totalCnt": totalCnt})
}

// DeleteQuestionMastery 删除用户对题目掌握度
func (uc *UserController) DeleteQuestionMastery(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	n, err := strconv.Atoi(c.Param("number"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题号"})
		return
	}
	if err := uc.db.Where("user_uuid = ? AND question_number = ?", targetUUID, n).Delete(&models.UserQuestionMastery{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// DeleteTagMastery 删除用户对标签掌握度
func (uc *UserController) DeleteTagMastery(c *gin.Context) {
	targetUUID := c.Param("uuid")
	operatorUUID := c.Query("operator_uuid")
	if !canAccessUserState(operatorUUID, targetUUID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}
	tag := strings.TrimSpace(c.Query("tag"))
	if tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag 必填"})
		return
	}
	if err := uc.db.Where("user_uuid = ? AND tag = ?", targetUUID, tag).Delete(&models.UserTagMastery{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GetRecommendationsV1 获取推荐题目（v1）
func (uc *UserController) GetRecommendationsV1(c *gin.Context) {
	var req struct {
		UserID         string  `json:"user_id" binding:"required"`
		Mode           string  `json:"mode"`
		TargetQuestion *int    `json:"target_question"`
		TargetTag      *string `json:"target_tag"`
		Limit          *int    `json:"limit"`
		Constraints    *struct {
			MasteryThreshold    *float64 `json:"mastery_threshold"`
			DifficultyTolerance *int     `json:"difficulty_tolerance"`
			MaxDepth            *int     `json:"max_depth"`
		} `json:"constraints"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if uc.graphService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "graph service unavailable"})
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "default"
	}

	limit := 20
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	masteryThreshold := 0.7
	difficultyTolerance := 1
	maxDepth := 6
	if req.Constraints != nil {
		if req.Constraints.MasteryThreshold != nil {
			masteryThreshold = *req.Constraints.MasteryThreshold
		}
		if req.Constraints.DifficultyTolerance != nil {
			difficultyTolerance = *req.Constraints.DifficultyTolerance
		}
		if req.Constraints.MaxDepth != nil {
			maxDepth = *req.Constraints.MaxDepth
		}
	}
	if masteryThreshold < 0 {
		masteryThreshold = 0
	}
	if masteryThreshold > 1 {
		masteryThreshold = 1
	}
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxDepth > 20 {
		maxDepth = 20
	}

	var qms []models.UserQuestionMastery
	if err := uc.db.Where("user_uuid = ?", req.UserID).Find(&qms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取用户掌握度失败"})
		return
	}
	masteryByQ := make(map[int]float64, len(qms))
	mastered := make(map[int]bool, len(qms))
	for _, m := range qms {
		masteryByQ[m.QuestionNumber] = m.Mastery
		if m.Mastery >= masteryThreshold {
			mastered[m.QuestionNumber] = true
		}
	}

	var tms []models.UserTagMastery
	_ = uc.db.Where("user_uuid = ?", req.UserID).Find(&tms).Error
	tagMastery := make(map[string]float64, len(tms))
	for _, m := range tms {
		tagMastery[m.Tag] = m.Mastery
	}

	difficultyRank := func(s string) int {
		s = strings.TrimSpace(strings.ToLower(s))
		s = strings.TrimSuffix(s, "级")
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
		switch s {
		case "easy", "简单":
			return 1
		case "medium", "中等":
			return 2
		case "hard", "困难":
			return 3
		case "expert", "非常困难", "地狱":
			return 4
		default:
			return 0
		}
	}
	absInt := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}

	baseRank := -1
	{
		var last models.UserQuestionMastery
		if err := uc.db.Where("user_uuid = ? AND accepted_count > 0 AND last_accepted_at IS NOT NULL", req.UserID).
			Order("last_accepted_at desc").First(&last).Error; err == nil {
			var q models.Question
			if err := uc.db.Where("question_number = ?", last.QuestionNumber).First(&q).Error; err == nil {
				baseRank = difficultyRank(q.Difficulty)
			}
		}
	}

	ctx := context.Background()
	qs, err := uc.graphService.ListQuestions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取图谱题目节点失败"})
		return
	}
	rels, err := uc.graphService.ListRelations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取图谱关系失败"})
		return
	}
	qNode := make(map[int]graph.QuestionNode, len(qs))
	for _, q := range qs {
		qNode[q.QuestionNumber] = q
	}

	type edge struct {
		to int
		w  float64
	}
	nextAdj := map[int][]edge{}
	simAdj := map[int][]edge{}
	simW := map[int]map[int]float64{}
	prereqs := map[int][]int{}
	prereqW := map[int]map[int]float64{}
	for _, r := range rels {
		switch r.RelationType {
		case graph.NEXT_LEVEL:
			nextAdj[r.FromQuestionNumber] = append(nextAdj[r.FromQuestionNumber], edge{to: r.ToQuestionNumber, w: r.Weight})
		case graph.SIMILAR:
			simAdj[r.FromQuestionNumber] = append(simAdj[r.FromQuestionNumber], edge{to: r.ToQuestionNumber, w: r.Weight})
			if simW[r.FromQuestionNumber] == nil {
				simW[r.FromQuestionNumber] = map[int]float64{}
			}
			simW[r.FromQuestionNumber][r.ToQuestionNumber] = r.Weight
		case graph.PREREQUISITE:
			prereqs[r.ToQuestionNumber] = append(prereqs[r.ToQuestionNumber], r.FromQuestionNumber)
			if prereqW[r.FromQuestionNumber] == nil {
				prereqW[r.FromQuestionNumber] = map[int]float64{}
			}
			prereqW[r.FromQuestionNumber][r.ToQuestionNumber] = r.Weight
		}
	}
	outPrereqCnt := map[int]int{}
	for from, m := range prereqW {
		outPrereqCnt[from] = len(m)
	}

	splitTagsLocal := func(tags string) []string { return splitTags(tags) }
	minTagMastery := func(tags []string) (string, float64) {
		bestTag := ""
		bestM := 1.0
		for _, t := range tags {
			m, ok := tagMastery[t]
			if !ok {
				m = 0
			}
			if bestTag == "" || m < bestM {
				bestTag = t
				bestM = m
			}
		}
		return bestTag, bestM
	}
	improvementScore := func(m float64) float64 {
		d := math.Abs(m - masteryThreshold)
		if d >= 0.1 {
			return 0
		}
		return 1 - (d / 0.1)
	}

	type cand struct {
		qn              int
		from            int
		relType         graph.RelationshipType
		edgeW           float64
		dist            int
		endpoint        int
		isConsolidation bool
		score           float64
		improve         float64
		consolidation   float64
		diversity       float64
		label           string
		pathNodes       []int
		edgeTypes       []string
		edgeWeights     []float64
	}
	candMap := map[int]*cand{}
	upsert := func(in *cand) {
		ex, ok := candMap[in.qn]
		if !ok || in.score > ex.score {
			candMap[in.qn] = in
		}
	}

	var nextStep map[int]int
	var endpoint map[int]int
	if mode == "target" {
		var targets []int
		if req.TargetQuestion != nil {
			targets = []int{*req.TargetQuestion}
		} else if req.TargetTag != nil {
			tag := strings.TrimSpace(*req.TargetTag)
			if tag != "" {
				for qn, q := range qNode {
					for _, t := range splitTagsLocal(q.Tags) {
						if t == tag {
							targets = append(targets, qn)
							break
						}
					}
				}
			}
		}
		if len(targets) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "目标模式需要 target_question 或 target_tag"})
			return
		}
		if req.TargetQuestion != nil {
			if _, ok := qNode[*req.TargetQuestion]; !ok {
				c.JSON(http.StatusNotFound, gin.H{"error": "目标题目不存在"})
				return
			}
		}

		queue := make([]int, 0, len(targets))
		dist := map[int]int{}
		nextStep = map[int]int{}
		endpoint = map[int]int{}
		for _, t := range targets {
			if _, ok := dist[t]; ok {
				continue
			}
			dist[t] = 0
			endpoint[t] = t
			queue = append(queue, t)
		}

		for head := 0; head < len(queue); head++ {
			cur := queue[head]
			if dist[cur] >= maxDepth {
				continue
			}
			for _, pre := range prereqs[cur] {
				if _, ok := dist[pre]; ok {
					continue
				}
				dist[pre] = dist[cur] + 1
				nextStep[pre] = cur
				endpoint[pre] = endpoint[cur]
				queue = append(queue, pre)
			}
		}

		for qn, d := range dist {
			if d == 0 {
				if req.TargetQuestion != nil {
					continue
				}
				if mastered[qn] {
					continue
				}
			} else if mastered[qn] {
				continue
			}
			base := 1.0 / float64(d+1)
			base += 0.02 * float64(outPrereqCnt[qn])
			cd := &cand{qn: qn, relType: graph.PREREQUISITE, edgeW: base, dist: d, endpoint: endpoint[qn]}
			upsert(cd)
		}
	} else {
		for qn := range mastered {
			for _, e := range nextAdj[qn] {
				if mastered[e.to] {
					continue
				}
				cd := &cand{qn: e.to, from: qn, relType: graph.NEXT_LEVEL, edgeW: e.w}
				upsert(cd)
			}
		}
		for qn := range mastered {
			for _, e := range simAdj[qn] {
				if mastered[e.to] {
					continue
				}
				if simW[e.to] == nil {
					continue
				}
				if _, ok := simW[e.to][qn]; !ok {
					continue
				}
				cd := &cand{qn: e.to, from: qn, relType: graph.SIMILAR, edgeW: e.w, isConsolidation: true}
				upsert(cd)
			}
		}
	}

	filtered := make([]*cand, 0, len(candMap))
	for _, cd := range candMap {
		qn := cd.qn
		node, ok := qNode[qn]
		if !ok {
			continue
		}
		if node.Status != "published" {
			continue
		}
		okPrereq := true
		for _, pre := range prereqs[qn] {
			if masteryByQ[pre] < masteryThreshold {
				okPrereq = false
				break
			}
		}
		if !okPrereq {
			continue
		}
		if baseRank >= 0 && difficultyTolerance >= 0 {
			if absInt(difficultyRank(node.Difficulty)-baseRank) > difficultyTolerance {
				continue
			}
		}

		m := masteryByQ[qn]
		cd.improve = improvementScore(m)
		cd.consolidation = 0
		if cd.isConsolidation {
			cd.consolidation = 1
		}
		tags := splitTagsLocal(node.Tags)
		label, minM := minTagMastery(tags)
		cd.label = label
		cd.diversity = 0
		if label != "" {
			cd.diversity = 1 - minM
		}
		base := cd.edgeW
		cd.score = 0.45*cd.improve + 0.10*cd.consolidation + 0.30*cd.diversity + 0.15*math.Min(base, 1)
		filtered = append(filtered, cd)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].score == filtered[j].score {
			return filtered[i].qn < filtered[j].qn
		}
		return filtered[i].score > filtered[j].score
	})

	reqConsolidation := int(math.Floor(float64(limit) * 0.05))
	if reqConsolidation < 0 {
		reqConsolidation = 0
	}

	groups := map[string][]*cand{}
	labels := make([]string, 0)
	labelSeen := map[string]bool{}
	for _, cd := range filtered {
		lb := cd.label
		if lb == "" {
			lb = "_"
		}
		groups[lb] = append(groups[lb], cd)
		if !labelSeen[lb] {
			labelSeen[lb] = true
			labels = append(labels, lb)
		}
	}
	sort.Slice(labels, func(i, j int) bool {
		ai, aj := groups[labels[i]][0].score, groups[labels[j]][0].score
		if ai == aj {
			return labels[i] < labels[j]
		}
		return ai > aj
	})

	selected := make([]*cand, 0, limit)
	used := map[int]bool{}
	for len(selected) < limit {
		progress := false
		for _, lb := range labels {
			lst := groups[lb]
			for len(lst) > 0 && used[lst[0].qn] {
				lst = lst[1:]
			}
			groups[lb] = lst
			if len(lst) == 0 {
				continue
			}
			selected = append(selected, lst[0])
			used[lst[0].qn] = true
			groups[lb] = lst[1:]
			progress = true
			if len(selected) >= limit {
				break
			}
		}
		if !progress {
			break
		}
	}

	if reqConsolidation > 0 {
		have := 0
		for _, cd := range selected {
			if cd.isConsolidation {
				have++
			}
		}
		if have < reqConsolidation {
			for _, cd := range filtered {
				if have >= reqConsolidation {
					break
				}
				if !cd.isConsolidation || used[cd.qn] {
					continue
				}
				if len(selected) < limit {
					selected = append(selected, cd)
					used[cd.qn] = true
					have++
					continue
				}
				for i := len(selected) - 1; i >= 0; i-- {
					if selected[i].isConsolidation {
						continue
					}
					used[selected[i].qn] = false
					selected[i] = cd
					used[cd.qn] = true
					have++
					break
				}
			}
		}
	}

	makePath := func(cd *cand) (string, []string, []float64) {
		if mode != "target" {
			if cd.from == 0 {
				return strconv.Itoa(cd.qn), []string{string(cd.relType)}, []float64{cd.edgeW}
			}
			return strconv.Itoa(cd.from) + "→" + strconv.Itoa(cd.qn), []string{string(cd.relType)}, []float64{cd.edgeW}
		}
		start := cd.qn
		end := cd.endpoint
		path := []int{start}
		cur := start
		for cur != end {
			nxt, ok := nextStep[cur]
			if !ok {
				break
			}
			path = append(path, nxt)
			cur = nxt
			if len(path) > maxDepth+2 {
				break
			}
		}
		et := make([]string, 0, len(path)-1)
		ew := make([]float64, 0, len(path)-1)
		for i := 0; i+1 < len(path); i++ {
			from, to := path[i], path[i+1]
			et = append(et, string(graph.PREREQUISITE))
			w := 1.0
			if prereqW[from] != nil {
				if ww, ok := prereqW[from][to]; ok {
					w = ww
				}
			}
			ew = append(ew, w)
		}
		parts := make([]string, 0, len(path))
		for _, n := range path {
			parts = append(parts, strconv.Itoa(n))
		}
		return strings.Join(parts, "→"), et, ew
	}

	items := make([]gin.H, 0, len(selected))
	for _, cd := range selected {
		p, et, ew := makePath(cd)
		items = append(items, gin.H{
			"question_id": strconv.Itoa(cd.qn),
			"score":       cd.score,
			"breakdown": gin.H{
				"improvement":   cd.improve,
				"consolidation": cd.consolidation,
				"diversity":     cd.diversity,
			},
			"explanation": gin.H{
				"path":         []string{p},
				"edge_types":   et,
				"edge_weights": ew,
				"confidence":   cd.score,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"recommendations": items})
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

// recordMastery 记录用户对题目和标签的掌握度
func (uc *UserController) recordMastery(userUUID string, question models.Question, accepted bool, now time.Time) error {
	if userUUID == "" || question.QuestionNumber == 0 {
		return nil
	}
	if err := uc.upsertQuestionMastery(userUUID, question.QuestionNumber, question.QuestionId, accepted, now); err != nil {
		return err
	}
	for _, tag := range splitTags(question.Tags) {
		_ = uc.upsertTagMastery(userUUID, tag, accepted, now)
	}
	return nil
}

// splitTags 分割标签字符串，处理逗号和中文逗号
func splitTags(tags string) []string {
	tags = strings.ReplaceAll(tags, "，", ",")
	parts := strings.Split(tags, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// upsertQuestionMastery 更新或插入用户对题目掌握度
func (uc *UserController) upsertQuestionMastery(userUUID string, questionNumber int, questionID string, accepted bool, now time.Time) error {
	var m models.UserQuestionMastery
	err := uc.db.Where("user_uuid = ? AND question_number = ?", userUUID, questionNumber).First(&m).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		m = models.UserQuestionMastery{UserUUID: userUUID, QuestionNumber: questionNumber}
	}
	if len(questionID) > 36 {
		questionID = questionID[:36]
	}
	if questionID == "" {
		questionID = strconv.Itoa(questionNumber)
	}
	if m.QuestionId != questionID {
		m.QuestionId = questionID
	}
	m.Attempts++
	m.LastSubmittedAt = &now
	if accepted {
		m.AcceptedCount++
		m.LastAcceptedAt = &now
	}
	if m.Attempts > 0 {
		m.Mastery = float64(m.AcceptedCount) / float64(m.Attempts)
	}
	return uc.db.Save(&m).Error
}

// upsertTagMastery 更新或插入用户对标签的掌握度
func (uc *UserController) upsertTagMastery(userUUID string, tag string, accepted bool, now time.Time) error {
	if tag == "" {
		return nil
	}
	var m models.UserTagMastery
	err := uc.db.Where("user_uuid = ? AND tag = ?", userUUID, tag).First(&m).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		m = models.UserTagMastery{UserUUID: userUUID, Tag: tag}
	}
	m.Attempts++
	m.LastSubmittedAt = &now
	if accepted {
		m.AcceptedCount++
		m.LastAcceptedAt = &now
	}
	if m.Attempts > 0 {
		m.Mastery = float64(m.AcceptedCount) / float64(m.Attempts)
	}
	return uc.db.Save(&m).Error
}

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

// requireOperatorUUID 从请求头或查询参数中获取操作人UUID
func (uc *UserController) requireOperatorUUID(c *gin.Context) (string, bool) {
	op := strings.TrimSpace(c.GetHeader("X-User-UUID"))
	if op == "" {
		op = strings.TrimSpace(c.Query("operator_uuid"))
	}
	if op == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return "", false
	}

	var user models.User
	if err := uc.db.Where("uuid = ?", op).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的登录状态"})
		return "", false
	}
	if user.Status != "" && user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return "", false
	}
	return op, true
}

// ListProblemSubmissions 获取题目提交记录（公开）
func (uc *UserController) ListProblemSubmissions(c *gin.Context) {
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
	if err := uc.db.Where("question_number = ?", qn).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	language := strings.TrimSpace(c.Query("language"))

	countQ := uc.db.Table("submissions").Joins("JOIN question ON question.id = submissions.question_id").Where("submissions.question_id = ?", question.Id)
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
	listQ := uc.db.Table("submissions").
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
func (uc *UserController) ListUserSubmissions(c *gin.Context) {
	opUUID, ok := uc.requireOperatorUUID(c)
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
			if err := uc.db.Where("question_number = ?", n).First(&q).Error; err == nil {
				questionID = &q.Id
			} else {
				questionID = &n
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "problem_id 无效"})
			return
		}
	}

	countQ := uc.db.Table("submissions").Joins("JOIN question ON question.id = submissions.question_id").Where("submissions.user_id = ?", targetUUID)
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
	listQ := uc.db.Table("submissions").
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

// canAccessUserState 判断用户是否有权限访问目标用户的状态
func canAccessUserState(operatorUUID string, targetUUID string) bool {
	if operatorUUID == "" || targetUUID == "" {
		return false
	}
	if operatorUUID == targetUUID {
		return true
	}
	return util.UserInstance.HasPermission(operatorUUID, "admin")
}
