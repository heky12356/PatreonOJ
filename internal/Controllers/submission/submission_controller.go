package submission

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/controllers/shared"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	assessmentsvc "patreonoj/internal/services/assessment"
	judgesvc "patreonoj/internal/services/judge"
	submissionsvc "patreonoj/internal/services/submission"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SubmissionController struct {
	db              *gorm.DB
	judgeService    *judgesvc.JudgeService
	queryService    *submissionsvc.SubmissionQueryService
	commandService  *submissionsvc.SubmissionCommandService
	submissionQueue chan *models.Submission
}

func NewSubmissionController(db *gorm.DB, ossClient *oss.OSS) *SubmissionController {
	queueSize := config.GlobalConfig.Judge.QueueSize
	bucket := config.GlobalConfig.OSS.BucketName
	assessmentService := assessmentsvc.NewAssessmentService(db)
	if bucket == "" {
		bucket = "patreon-oj-cases"
	}

	judgeService := judgesvc.NewJudgeService(&config.GlobalConfig.Judge, db, ossClient, bucket, assessmentService)
	controller := &SubmissionController{
		db:              db,
		judgeService:    judgeService,
		queryService:    submissionsvc.NewSubmissionQueryService(db),
		commandService:  submissionsvc.NewSubmissionCommandService(db, judgeService),
		submissionQueue: make(chan *models.Submission, queueSize),
	}

	go controller.consumeSubmissions()
	return controller
}

type SubmitRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	QuestionPK int    `json:"question_pk"`
	QuestionID string `json:"question_id"`
	Code       string `json:"code" binding:"required"`
}

func (sc *SubmissionController) consumeSubmissions() {
	for submission := range sc.submissionQueue {
		submission.Status = "processing"
		if err := sc.db.Save(submission).Error; err != nil {
			log.Printf("save submission processing status failed, submission=%s err=%v", submission.ID, err)
			continue
		}

		if err := sc.judgeService.JudgeCode(submission); err != nil {
			log.Printf("judge failed, submission=%s err=%v", submission.ID, err)
			submission.Status = "error"
			submission.ErrorCode = submissionsvc.EnsureSubmissionErrorCode(err)
			submission.ErrorMsg = err.Error()
			submission.Results = ""
		}

		if err := sc.db.Save(submission).Error; err != nil {
			log.Printf("save submission result failed, submission=%s err=%v", submission.ID, err)
		}

		allCorrect := true
		results := submissionsvc.ParseSubmissionResults(submission.Results)
		for _, r := range results {
			if !r.IsCorrect {
				allCorrect = false
				break
			}
		}
		if !allCorrect || submission.Status != "completed" || len(results) == 0 {
			continue
		}

		rec := models.UserSolvedQuestion{
			UserUUID:   submission.UserID,
			QuestionID: submission.QuestionID,
			SolvedAt:   time.Now(),
		}
		if err := sc.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_uuid"}, {Name: "question_id"}},
			DoNothing: true,
		}).Create(&rec).Error; err != nil {
			log.Printf("insert user solved record failed, user=%s question_pk=%d err=%v", rec.UserUUID, rec.QuestionID, err)
		}
	}
}

func (sc *SubmissionController) SubmitCode(c *gin.Context) {
	var submitRequest SubmitRequest
	if err := c.ShouldBindJSON(&submitRequest); err != nil {
		shared.APIError(c, http.StatusBadRequest, err.Error())
		return
	}

	var (
		result *submissionsvc.SubmissionCreateResult
		err    error
	)
	if submitRequest.QuestionPK > 0 {
		result, err = sc.commandService.CreateSubmission(submitRequest.UserID, submitRequest.QuestionPK, submitRequest.Code)
	} else if qid := strings.TrimSpace(submitRequest.QuestionID); qid != "" {
		result, err = sc.commandService.CreateSubmissionByQuestionID(submitRequest.UserID, qid, submitRequest.Code)
	} else {
		shared.APIError(c, http.StatusBadRequest, "question_pk is required")
		return
	}

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
	if err != nil {
		shared.APIError(c, http.StatusInternalServerError, "create submission failed")
		return
	}

	submission := result.Submission
	sc.submissionQueue <- submission

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
	submissionID := c.Param("id")
	response, err := sc.commandService.BuildSubmissionResult(submissionID)
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
	shared.APIOK(c, http.StatusOK, msg, response)
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
	result, err := sc.queryService.ListProblemSubmissions(questionID, status, language, page, size)
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
	opUUID, ok := shared.RequireOperatorUUID(sc.db, c)
	if !ok {
		return
	}
	isAdmin := util.UserInstance.HasPermission(opUUID, "admin")

	targetUUID := strings.TrimSpace(c.Param("user_id"))
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
