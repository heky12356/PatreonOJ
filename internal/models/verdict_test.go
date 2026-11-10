package models

import "testing"

func TestVerdictsSeparateLifecycleAndExecution(t *testing.T) {
	for _, tc := range []struct {
		name       string
		submission Submission
		want       string
	}{
		{"accepted", Submission{Status: "completed", Results: `[{"is_correct":true}]`}, "AC"},
		{"wrong-answer", Submission{Status: "completed", Results: `[{"is_correct":false}]`}, "WA"},
		{"hidden-runtime-error", Submission{Status: "completed", Results: `[{"is_correct":false,"is_hidden":true,"verdict":"RE"}]`}, "RE"},
		{"time-limit", Submission{Status: "completed", Verdict: "TLE"}, "TLE"},
		{"compile-error", Submission{Status: "error", ErrorCode: "E002"}, "CE"},
		{"system-error", Submission{Status: "error", ErrorCode: "E005"}, "SE"},
		{"queued", Submission{Status: "pending", Verdict: "AC"}, "PENDING"},
		{"processing", Submission{Status: "processing"}, "JUDGING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.submission.ResultVerdict(); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	if got := (TestCaseResult{Verdict: "AC", ActualOutput: "Time Limit Exceeded"}).ResultVerdict(); got != "WA" {
		t.Fatal("stdout was misclassified as execution failure")
	}
}
