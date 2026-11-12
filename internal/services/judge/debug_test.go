package judge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDebugPreservesOutputErrorsAndEmptyInput(t *testing.T) {
	for _, test := range []struct {
		status          string
		exit            int
		stdout, verdict string
	}{{"Accepted", 0, "3\n", "OK"}, {"Accepted", 7, "before crash\n", "RE"}, {"Time Limit Exceeded", 0, "partial\n", "TLE"}} {
		t.Run(test.verdict, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Cmd []CmdRequest `json:"cmd"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if len(body.Cmd) != 1 || body.Cmd[0].Files[0].Content == nil || *body.Cmd[0].Files[0].Content != "" {
					t.Error("empty custom input was not preserved")
				}
				json.NewEncoder(w).Encode([]CmdResponse{{Status: test.status, ExitStatus: test.exit, Files: map[string]string{"stdout": test.stdout, "stderr": "diagnostic"}, Time: 2000000, Memory: 4096}})
			}))
			defer server.Close()
			client := NewGoJudgeClient(server.URL, 2)
			expected := "3\n"
			result, err := client.DebugContext(context.Background(), "print(3)", "python", "", &expected, 1000, 128)
			if err != nil || result.Verdict != test.verdict || result.Stdout != test.stdout || result.Stderr != "diagnostic" || result.ExitCode != test.exit {
				t.Fatalf("result: %+v %v", result, err)
			}
			if test.verdict == "OK" && result.Comparison != "match" {
				t.Fatal("expected-output comparison missing")
			}
		})
	}
}
