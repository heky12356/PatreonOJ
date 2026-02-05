package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MasteryController 处理用户掌握度相关的请求
type MasteryController struct {
	db *gorm.DB
}

// NewMasteryController 创建掌握度控制器
func NewMasteryController(db *gorm.DB) *MasteryController {
	return &MasteryController{db: db}
}

// MasteryEventRequest 提交用户对题目或标签的掌握度事件
type MasteryEventRequest struct {
	QuestionNumber int  `json:"question_number" binding:"required"`
	Accepted       bool `json:"accepted"`
}

// SubmitMasteryEvent 提交用户对题目或标签的掌握度事件
func (mc *MasteryController) SubmitMasteryEvent(c *gin.Context) {
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
	if err := mc.db.Where("question_number = ?", req.QuestionNumber).First(&question).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}
	if err := mc.recordMastery(targetUUID, question, req.Accepted, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入掌握度失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ListQuestionMastery 获取用户对所有题目掌握度
func (mc *MasteryController) ListQuestionMastery(c *gin.Context) {
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

	q := mc.db.Model(&models.UserQuestionMastery{}).Where("user_uuid = ?", targetUUID)
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
func (mc *MasteryController) ListTagMastery(c *gin.Context) {
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

	q := mc.db.Model(&models.UserTagMastery{}).Where("user_uuid = ?", targetUUID)
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
func (mc *MasteryController) DeleteQuestionMastery(c *gin.Context) {
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
	if err := mc.db.Where("user_uuid = ? AND question_number = ?", targetUUID, n).Delete(&models.UserQuestionMastery{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// DeleteTagMastery 删除用户对标签掌握度
func (mc *MasteryController) DeleteTagMastery(c *gin.Context) {
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
	if err := mc.db.Where("user_uuid = ? AND tag = ?", targetUUID, tag).Delete(&models.UserTagMastery{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// recordMastery 记录用户对题目和标签的掌握度
func (mc *MasteryController) recordMastery(userUUID string, question models.Question, accepted bool, now time.Time) error {
	if userUUID == "" || question.QuestionNumber == 0 {
		return nil
	}
	if err := mc.upsertQuestionMastery(userUUID, question.QuestionNumber, question.QuestionId, accepted, now); err != nil {
		return err
	}
	for _, tag := range splitTags(question.Tags) {
		_ = mc.upsertTagMastery(userUUID, tag, accepted, now)
	}
	return nil
}

// upsertQuestionMastery 更新或插入用户对题目掌握度
func (mc *MasteryController) upsertQuestionMastery(userUUID string, questionNumber int, questionID string, accepted bool, now time.Time) error {
	var m models.UserQuestionMastery
	err := mc.db.Where("user_uuid = ? AND question_number = ?", userUUID, questionNumber).First(&m).Error
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
	return mc.db.Save(&m).Error
}

// upsertTagMastery 更新或插入用户对标签的掌握度
func (mc *MasteryController) upsertTagMastery(userUUID string, tag string, accepted bool, now time.Time) error {
	if tag == "" {
		return nil
	}
	var m models.UserTagMastery
	err := mc.db.Where("user_uuid = ? AND tag = ?", userUUID, tag).First(&m).Error
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
	return mc.db.Save(&m).Error
}
