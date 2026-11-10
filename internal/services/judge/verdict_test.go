package judge

import "testing"

func TestExecutionVerdictDoesNotDependOnUserStdout(t *testing.T) {
	for _, tc := range []struct {
		status string
		exit   int
		want   string
	}{
		{"Accepted", 0, "AC"}, {"Accepted", 1, "RE"}, {"Time Limit Exceeded", 0, "TLE"}, {"Memory Limit Exceeded", 0, "MLE"}, {"Output Limit Exceeded", 0, "OLE"}, {"Signalled", 0, "RE"}, {"Internal Error", 0, "SE"},
	} {
		result := parseResult(CmdResponse{Status: tc.status, ExitStatus: tc.exit, Files: map[string]string{"stdout": "Time Limit Exceeded"}}, "")
		if result.Verdict != tc.want {
			t.Fatalf("%s exit %d: got %s want %s", tc.status, tc.exit, result.Verdict, tc.want)
		}
	}
}
