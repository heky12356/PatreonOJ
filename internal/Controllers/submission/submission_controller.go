package submission

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"patreonoj/internal/Controllers/shared"
	"patreonoj/internal/config"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	assessmentsvc "patreonoj/internal/services/assessment"
	judgesvc "patreonoj/internal/services/judge"
	submissionsvc "patreonoj/internal/services/submission"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubmissionController struct {
	db             *gorm.DB
	judgeService   *judgesvc.JudgeService
	queryService   *submissionsvc.SubmissionQueryService
	commandService *submissionsvc.SubmissionCommandService
	worker         *submissionsvc.SubmissionWorker
	queueLimit     int
}

func NewSubmissionController(db *gorm.DB, ossClient *oss.OSS) *SubmissionController {
	queueSize := config.GlobalConfig.Judge.QueueSize
	if queueSize <= 0 {
		queueSize = 100
	}
	bucket := config.GlobalConfig.OSS.BucketName
	assessmentService := assessmentsvc.NewAssessmentService(db)
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	judgeService := judgesvc.NewJudgeService(&config.GlobalConfig.Judge, db, ossClient, bucket, assessmentService)
	controller := &SubmissionController{
		db:             db,
		judgeService:   judgeService,
		queryService:   submissionsvc.NewSubmissionQueryService(db),
		commandService: submissionsvc.NewSubmissionCommandService(db, judgeService),
		queueLimit:     queueSize,
	}
	controller.worker = submissionsvc.NewSubmissionWorker(db, judgeService)
	return controller
}

type SubmitRequest struct {
	// UserID     string `json:"user_id" binding:"required"` 不使用前端发的id而是通过c.get拿到的cookie中解析的user的id
	QuestionPK int    `json:"question_pk"`
	QuestionID string `json:"question_id"`
	Code       string `json:"code" binding:"required"`
	Language   string `json:"language"`
}

func (sc *SubmissionController) Shutdown(ctx context.Context) error {
	return sc.worker.Shutdown(ctx)
}

func (sc *SubmissionController) SubmitCode(c *gin.Context) {
	currentUser, ok := c.Get("currentUser")
	user, valid := currentUser.(models.User)
	if !ok || !valid {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	var submitRequest SubmitRequest
	if err := c.ShouldBindJSON(&submitRequest); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	if submitRequest.QuestionPK <= 0 && strings.TrimSpace(submitRequest.QuestionID) == "" {
		shared.APIError(c, http.StatusBadRequest, "question_pk is required")
		return
	}
	language := strings.ToLower(strings.TrimSpace(submitRequest.Language))
	if language != "" && language != "go" && language != "cpp" && language != "python" && language != "java" {
		shared.APIError(c, http.StatusBadRequest, "unsupported language")
		return
	}
	result, err := sc.commandService.CreateSubmissionContext(c.Request.Context(), user.UUID, submitRequest.QuestionPK,
		submitRequest.QuestionID, submitRequest.Code, language, sc.queueLimit)

	if errors.Is(err, submissionsvc.ErrSubmissionUserNotFound) {
		shared.APIError(c, http.StatusNotFound, "user not found")
		return
	}
	if errors.Is(err, submissionsvc.ErrSubmissionUserDisabled) {
		shared.APIError(c, http.StatusForbidden, "user is disabled")
		return
	}
	if errors.Is(err, submissionsvc.ErrSubmissionQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if errors.Is(err, submissionsvc.ErrSubmissionCodeInvalid) || errors.Is(err, submissionsvc.ErrSubmissionQuestionAmbiguous) {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, submissionsvc.ErrSubmissionForbidden) {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}
	if errors.Is(err, submissionsvc.ErrSubmissionQueueFull) {
		shared.APIError(c, http.StatusTooManyRequests, "submission queue is full, please retry later")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "create submission failed")
		return
	}

	submission := result.Submission
	sc.worker.Notify()

	shared.APIOK(c, http.StatusOK, "submission queued", gin.H{
		"submission_id": submission.ID,
		"user_id":       submission.UserID,
		"question_id":   result.QuestionID,
		"question_pk":   result.QuestionPK,
		"status":        submission.Status,
		"created_at":    submission.CreatedAt,
	})
}

func (sc *SubmissionController) GetSubmissionResult(c *gin.Context) {
	currentUser, ok := c.Get("currentUser")
	user, valid := currentUser.(models.User)
	if !ok || !valid {
		shared.APIError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	submissionID := c.Param("id")
	response, err := sc.commandService.BuildSubmissionResultForViewer(submissionID, user)
	if errors.Is(err, submissionsvc.ErrSubmissionForbidden) {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}
	if errors.Is(err, submissionsvc.ErrSubmissionNotFound) {
		shared.APIError(c, http.StatusNotFound, "submission not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query failed")
		return
	}

	msg := "ok"
	if m, ok := response["message"].(string); ok && m != "" {
		msg = m
	}
	shared.APIOK(c, http.StatusOK, msg, gin.H{"data": response})
}

func (sc *SubmissionController) ListProblemSubmissions(c *gin.Context) {
	questionID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || questionID <= 0 {
		shared.APIError(c, http.StatusBadRequest, "invalid question id")
		return
	}

	page, size, err := shared.ParsePageSizeQuery(c, "page", "size", 1, 20, 100)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	language := strings.TrimSpace(c.Query("language"))
	viewerUUID, isAdmin := "", false
	if currentUser, ok := c.Get("currentUser"); ok {
		if user, valid := currentUser.(models.User); valid {
			viewerUUID, isAdmin = user.UUID, util.UserInstance.HasPermission(user, "admin")
		}
	}
	result, err := sc.queryService.ListProblemSubmissionsForViewer(questionID, status, language, page, size, viewerUUID, isAdmin)
	if errors.Is(err, submissionsvc.ErrSubmissionQuestionNotFound) {
		shared.APIError(c, http.StatusNotFound, "question not found")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{
		"total": result.Total,
		"page":  result.Page,
		"size":  result.Size,
		"pages": result.Pages,
		"items": result.Items,
	})
}

func (sc *SubmissionController) ListUserSubmissions(c *gin.Context) {
	user, _ := c.Get("currentUser")
	opUUID := user.(models.User).UUID
	isAdmin := util.UserInstance.HasPermission(user.(models.User), "admin")

	targetUUID := strings.TrimSpace(c.Param("user_id"))
	if id, err := strconv.Atoi(targetUUID); err == nil && id > 0 {
		var target models.User
		if err := sc.db.First(&target, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				shared.APIError(c, http.StatusNotFound, "user not found")
			} else {
				shared.APIError(c, http.StatusInternalServerError, "query user failed")
			}
			return
		}
		targetUUID = target.UUID
	}
	if targetUUID == "" {
		shared.APIError(c, http.StatusBadRequest, "invalid user_id")
		return
	}
	if opUUID != targetUUID && !isAdmin {
		shared.APIError(c, http.StatusForbidden, "permission denied")
		return
	}

	page, size, err := shared.ParsePageSizeQuery(c, "page", "size", 1, 20, 100)
	if err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	language := strings.TrimSpace(c.Query("language"))
	problemID := strings.TrimSpace(c.Query("problem_id"))
	result, err := sc.queryService.ListUserSubmissions(targetUUID, problemID, status, language, page, size)
	if errors.Is(err, submissionsvc.ErrSubmissionProblemIDInvalid) {
		shared.APIError(c, http.StatusBadRequest, "invalid problem_id")
		return
	}
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "query failed")
		return
	}
	shared.APIOK(c, http.StatusOK, "ok", gin.H{
		"total": result.Total,
		"page":  result.Page,
		"size":  result.Size,
		"pages": result.Pages,
		"items": result.Items,
	})
}
