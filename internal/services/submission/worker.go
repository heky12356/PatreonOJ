package submission

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

const maxJudgeAttempts = 3

type JudgeRunner interface {
	JudgeCodeContext(context.Context, *models.Submission) error
}

// SubmissionWorker uses persisted submissions as its queue. The channel only
// wakes the worker; a notification may be dropped without losing a submission.
type SubmissionWorker struct {
	db       *gorm.DB
	judge    JudgeRunner
	notify   chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
	deadline time.Duration
}

func NewSubmissionWorker(db *gorm.DB, judge JudgeRunner) *SubmissionWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &SubmissionWorker{
		db: db, judge: judge, notify: make(chan struct{}, 1),
		cancel: cancel, done: make(chan struct{}), deadline: 5 * time.Minute,
	}
	go w.run(ctx)
	return w
}

func (w *SubmissionWorker) Notify() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

func (w *SubmissionWorker) Shutdown(ctx context.Context) error {
	w.stopOnce.Do(w.cancel)
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *SubmissionWorker) run(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastRecovery := time.Time{}
	for ctx.Err() == nil {
		if time.Since(lastRecovery) >= 30*time.Second {
			if err := w.recoverStale(ctx); err != nil && ctx.Err() == nil {
				log.Printf("recover stale submissions failed: %v", err)
			}
			lastRecovery = time.Now()
		}
		submission, err := w.claim(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("claim submission failed: %v", err)
			}
		} else if submission != nil {
			w.process(ctx, submission)
			continue
		}
		select {
		case <-ctx.Done():
		case <-w.notify:
		case <-ticker.C:
		}
	}
}

func (w *SubmissionWorker) recoverStale(ctx context.Context) error {
	db := w.db.WithContext(ctx)
	cutoff := time.Now().Add(-w.deadline - 30*time.Second)
	if err := db.Model(&models.Submission{}).
		Where("status = ? AND updated_at < ? AND attempts >= ?", "processing", cutoff, maxJudgeAttempts).
		Updates(map[string]interface{}{"status": "error", "error_code": "E005", "error_msg": "judge worker did not complete before its lease expired"}).Error; err != nil {
		return err
	}
	return db.Model(&models.Submission{}).
		Where("status = ? AND updated_at < ? AND attempts < ?", "processing", cutoff, maxJudgeAttempts).
		Updates(map[string]interface{}{"status": "pending", "error_code": "", "error_msg": ""}).Error
}

func (w *SubmissionWorker) claim(ctx context.Context) (*models.Submission, error) {
	db := w.db.WithContext(ctx)
	var candidate models.Submission
	err := db.Where("status = ?", "pending").Order("created_at ASC, id ASC").First(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if candidate.Attempts >= maxJudgeAttempts {
		return nil, db.Model(&models.Submission{}).Where("id = ? AND status = ?", candidate.ID, "pending").
			Updates(map[string]interface{}{"status": "error", "error_code": "E005", "error_msg": "judge retry limit exceeded"}).Error
	}
	result := db.Model(&models.Submission{}).
		Where("id = ? AND status = ? AND attempts = ?", candidate.ID, "pending", candidate.Attempts).
		Updates(map[string]interface{}{
			"status": "processing", "attempts": gorm.Expr("attempts + 1"),
			"error_code": "", "error_msg": "",
		})
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, result.Error
	}
	// Read the timestamp back so the lease matches the database's precision.
	if err := db.First(&candidate, "id = ?", candidate.ID).Error; err != nil {
		return nil, err
	}
	return &candidate, nil
}

func (w *SubmissionWorker) process(parent context.Context, submission *models.Submission) {
	lease := submission.UpdatedAt
	ctx, cancel := context.WithTimeout(parent, w.deadline)
	err := func() (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("judge worker panic: %v", p)
			}
		}()
		return w.judge.JudgeCodeContext(ctx, submission)
	}()
	cancel()
	if err == nil {
		return
	}
	updates := map[string]interface{}{
		"status": "error", "error_code": EnsureSubmissionErrorCode(err), "error_msg": err.Error(), "results": "",
	}
	if parent.Err() != nil {
		// A graceful service stop should not consume a retry or strand work.
		updates["status"], updates["error_code"], updates["error_msg"] = "pending", "", ""
		updates["attempts"] = gorm.Expr("attempts - 1")
	} else if EnsureSubmissionErrorCode(err) == "E005" && submission.Attempts < maxJudgeAttempts {
		updates["status"], updates["error_code"], updates["error_msg"] = "pending", "", ""
	}
	// Use a fresh bounded context to release the lease even during shutdown.
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer saveCancel()
	if saveErr := w.db.WithContext(saveCtx).Model(&models.Submission{}).
		Where("id = ? AND status = ? AND updated_at = ?", submission.ID, "processing", lease).
		Updates(updates).Error; saveErr != nil {
		log.Printf("save submission failure failed, submission=%s err=%v", submission.ID, saveErr)
	}
	if parent.Err() == nil && updates["status"] == "pending" {
		timer := time.NewTimer(time.Duration(submission.Attempts) * time.Second)
		defer timer.Stop()
		select {
		case <-parent.Done():
		case <-timer.C:
		}
	}
}
