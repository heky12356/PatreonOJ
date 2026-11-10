package models

import (
	"encoding/json"
	"strings"
)

// ResultVerdict keeps the worker lifecycle separate from the judge's result.
func (s Submission) ResultVerdict() string {
	switch s.Status {
	case "pending":
		return "PENDING"
	case "processing", "judging":
		return "JUDGING"
	case "error":
		switch s.ErrorCode {
		case "E002":
			return "CE"
		case "E003":
			return "TLE"
		case "E004":
			return "MLE"
		default:
			return "SE"
		}
	case "completed", "accepted":
		if validVerdict(s.Verdict) {
			return strings.ToUpper(s.Verdict)
		}
		var results []TestCaseResult
		if json.Unmarshal([]byte(s.Results), &results) != nil || len(results) == 0 {
			return "SE"
		}
		for _, result := range results {
			if !result.IsCorrect {
				return result.ResultVerdict()
			}
		}
		return "AC"
	default:
		return "SE"
	}
}

func (r TestCaseResult) ResultVerdict() string {
	if r.IsCorrect {
		return "AC"
	}
	if validVerdict(r.Verdict) && strings.ToUpper(r.Verdict) != "AC" {
		return strings.ToUpper(r.Verdict)
	}
	// Older records did not retain an execution verdict. User output is never
	// interpreted as an error: printing "Time Limit Exceeded" can simply be WA.
	return "WA"
}

func validVerdict(value string) bool {
	switch strings.ToUpper(value) {
	case "AC", "WA", "RE", "TLE", "MLE", "OLE", "CE", "SE":
		return true
	default:
		return false
	}
}

// BackfillSubmissionVerdicts processes one result body at a time to keep old
// deployments bounded even when individual submissions contain large results.
func BackfillSubmissionVerdicts() error {
	for {
		var ids []string
		if err := DB.Model(&Submission{}).Where("(verdict = ? OR verdict IS NULL) AND status IN ?", "", []string{"completed", "error"}).Limit(100).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		for _, id := range ids {
			var submission Submission
			if err := DB.Select("id", "status", "verdict", "results", "error_code").First(&submission, "id = ?", id).Error; err != nil {
				return err
			}
			if err := DB.Model(&Submission{}).Where("id = ? AND (verdict = ? OR verdict IS NULL)", id, "").UpdateColumn("verdict", submission.ResultVerdict()).Error; err != nil {
				return err
			}
		}
	}
}
