package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"patreonoj/internal/models"
)

const defaultJudgeEnv = "PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

type GoJudgeClient struct {
	APIURL     string
	Token      string
	HTTPClient *http.Client
}

func NewGoJudgeClient(apiURL string, timeout int) *GoJudgeClient {
	if timeout <= 0 {
		timeout = 15
	}
	return &GoJudgeClient{APIURL: apiURL, HTTPClient: &http.Client{Timeout: time.Duration(timeout) * time.Second}}
}

type CmdRequest struct {
	Args          []string           `json:"args"`
	Env           []string           `json:"env,omitempty"`
	Files         []*CmdFile         `json:"files,omitempty"`
	CPULimit      uint64             `json:"cpuLimit,omitempty"`
	ClockLimit    uint64             `json:"clockLimit,omitempty"`
	MemoryLimit   uint64             `json:"memoryLimit,omitempty"`
	ProcLimit     uint64             `json:"procLimit,omitempty"`
	StackLimit    uint64             `json:"stackLimit,omitempty"`
	CopyIn        map[string]CmdFile `json:"copyIn,omitempty"`
	CopyOut       []string           `json:"copyOut,omitempty"`
	CopyOutCached []string           `json:"copyOutCached,omitempty"`
	CopyOutDir    string             `json:"copyOutDir,omitempty"`
}

type CmdFile struct {
	Name    string  `json:"name,omitempty"`
	Content *string `json:"content,omitempty"`
	Src     *string `json:"src,omitempty"`
	FileID  *string `json:"fileId,omitempty"`
	Max     int64   `json:"max,omitempty"`
}

type CmdResponse struct {
	Status     string            `json:"status"`
	ExitStatus int               `json:"exitStatus"`
	Error      string            `json:"error"`
	Time       uint64            `json:"time"`
	Memory     uint64            `json:"memory"`
	RunTime    uint64            `json:"runTime"`
	Files      map[string]string `json:"files"`
	FileIds    map[string]string `json:"fileIds"`
}

// CompileError is a user-code failure, so it must not trigger a second compilation locally.
type CompileError struct{ Message string }

func (e *CompileError) Error() string { return "Compile Error: " + e.Message }

func (c *GoJudgeClient) Run(code, language string, inputs []string, timeLimitMs, memoryLimitMB int64) ([]models.TestCaseResult, error) {
	return c.RunContext(context.Background(), code, language, inputs, timeLimitMs, memoryLimitMB)
}

func (c *GoJudgeClient) RunContext(ctx context.Context, code, language string, inputs []string, timeLimitMs, memoryLimitMB int64) ([]models.TestCaseResult, error) {
	if len(inputs) == 0 {
		return nil, errors.New("no test cases supplied")
	}
	if timeLimitMs <= 0 || memoryLimitMB <= 0 || timeLimitMs > 86_400_000 || memoryLimitMB > 1_048_576 {
		return nil, errors.New("invalid judge resource limits")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cpu, clock, memory := uint64(timeLimitMs)*1_000_000, uint64(timeLimitMs)*3_000_000, uint64(memoryLimitMB)*1024*1024
	var artifact, artifactID string
	switch language {
	case "cpp", "go", "java":
		var err error
		artifact, artifactID, err = c.compile(ctx, code, language)
		if err != nil {
			return nil, err
		}
		defer c.deleteCachedFile(artifactID)
	case "python":
	default:
		return nil, fmt.Errorf("unsupported language for go-judge: %s", language)
	}
	cmds := make([]CmdRequest, 0, len(inputs))
	for _, input := range inputs {
		in := input
		cmd := CmdRequest{Env: []string{defaultJudgeEnv, "PYTHONIOENCODING=utf-8"}, Files: judgeFiles(&in), CPULimit: cpu, ClockLimit: clock, MemoryLimit: memory, ProcLimit: 64}
		switch language {
		case "python":
			cmd.Args = []string{"python3", "main.py"}
			cmd.CopyIn = map[string]CmdFile{"main.py": {Content: &code}}
		case "java":
			cmd.Args = []string{"java", "-XX:+UseSerialGC", fmt.Sprintf("-Xmx%dm", max(int64(16), memoryLimitMB/2)), "-cp", "main.jar", "Main"}
			cmd.CopyIn = map[string]CmdFile{artifact: {FileID: &artifactID}}
		default:
			cmd.Args = []string{"./" + artifact}
			cmd.CopyIn = map[string]CmdFile{artifact: {FileID: &artifactID}}
		}
		cmds = append(cmds, cmd)
	}
	responses, err := c.request(ctx, cmds)
	if err != nil {
		return nil, err
	}
	results := make([]models.TestCaseResult, len(inputs))
	for i, response := range responses {
		if response.Status == "Internal Error" || response.Status == "File Error" {
			return nil, fmt.Errorf("go-judge service error: %s %s", response.Status, response.Error)
		}
		results[i] = parseResult(response, inputs[i])
	}
	return results, nil
}

func judgeFiles(input *string) []*CmdFile {
	return []*CmdFile{{Content: input}, {Name: "stdout", Max: 1024 * 1024}, {Name: "stderr", Max: 64 * 1024}}
}

func (c *GoJudgeClient) compile(ctx context.Context, code, language string) (string, string, error) {
	artifact := "main"
	cmd := CmdRequest{Env: []string{defaultJudgeEnv}, Files: judgeFiles(new(string)), CPULimit: 10_000_000_000, ClockLimit: 15_000_000_000, MemoryLimit: 512 * 1024 * 1024, ProcLimit: 64}
	switch language {
	case "cpp":
		cmd.Args = []string{"g++", "main.cpp", "-o", artifact, "-O2", "-std=c++17"}
		cmd.CopyIn = map[string]CmdFile{"main.cpp": {Content: &code}}
	case "go":
		// Cold Go builds also compile the standard library. This budget is
		// separate from the problem's execution limit and remains bounded.
		cmd.CPULimit, cmd.ClockLimit = 45_000_000_000, 90_000_000_000
		cmd.Args = []string{"go", "build", "-p", "2", "-o", artifact, "main.go"}
		cmd.Env = append(cmd.Env, "GOCACHE=/tmp/go-cache", "GOMODCACHE=/tmp/go-mod", "GO111MODULE=off", "GOTOOLCHAIN=local", "GOMAXPROCS=2")
		cmd.CopyIn = map[string]CmdFile{"main.go": {Content: &code}}
	case "java":
		artifact = "main.jar"
		// /tmp and /w are separate sandbox mounts. Keep jar's temporary archive
		// in the work directory so packaging does not require a cross-mount move.
		cmd.Args = []string{"sh", "-c", "mkdir -p classes && javac -d classes Main.java && jar -J-Djava.io.tmpdir=. cf main.jar -C classes ."}
		cmd.CopyIn = map[string]CmdFile{"Main.java": {Content: &code}}
	}
	cmd.CopyOutCached = []string{artifact + "?"}
	responses, err := c.request(ctx, []CmdRequest{cmd})
	if err != nil {
		return "", "", fmt.Errorf("go-judge execution request failed: %w", err)
	}
	response := responses[0]
	if response.Status == "Internal Error" || response.Status == "File Error" {
		return "", "", fmt.Errorf("go-judge service error: %s %s", response.Status, response.Error)
	}
	if response.Status != "Accepted" || response.ExitStatus != 0 {
		for _, id := range response.FileIds {
			c.deleteCachedFile(id)
		}
		return "", "", &CompileError{Message: strings.TrimSpace(response.Status + " " + response.Error + " " + response.Files["stderr"])}
	}
	id := response.FileIds[artifact]
	if id == "" {
		id = response.FileIds[artifact+"?"]
	}
	if id == "" {
		return "", "", errors.New("go-judge invalid response: no executable fileId returned")
	}
	return artifact, id, nil
}

func (c *GoJudgeClient) request(ctx context.Context, cmds []CmdRequest) ([]CmdResponse, error) {
	body, err := json.Marshal(map[string]interface{}{"cmd": cmds})
	if err != nil {
		return nil, err
	}
	results, status, err := c.post(ctx, c.APIURL, body)
	if err == nil && status == http.StatusNotFound {
		if fallback, ok := fallbackGoJudgeURL(c.APIURL, status); ok {
			results, status, err = c.post(ctx, fallback, body)
		}
	}
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("go-judge api error: %d", status)
	}
	if len(results) != len(cmds) {
		return nil, fmt.Errorf("go-judge response count mismatch: got=%d want=%d", len(results), len(cmds))
	}
	return results, nil
}

func (c *GoJudgeClient) post(ctx context.Context, address string, body []byte) ([]CmdResponse, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return nil, response.StatusCode, nil
	}
	// Bound malformed upstream responses without buffering arbitrary amounts of data.
	data, err := io.ReadAll(io.LimitReader(response.Body, 64*1024*1024+1))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if len(data) > 64*1024*1024 {
		return nil, response.StatusCode, errors.New("go-judge response exceeds size limit")
	}
	var results []CmdResponse
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, response.StatusCode, fmt.Errorf("invalid go-judge response: %w", err)
	}
	return results, response.StatusCode, nil
}

func (c *GoJudgeClient) deleteCachedFile(id string) {
	if id == "" {
		return
	}
	address, err := url.Parse(strings.TrimRight(c.APIURL, "/"))
	if err != nil {
		return
	}
	address.Path = strings.TrimSuffix(address.Path, "/run") + "/file/" + url.PathEscape(id)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, address.String(), nil)
	if err != nil {
		return
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTPClient.Do(req)
	if err != nil {
		log.Printf("go-judge cached file cleanup failed: %v", err)
		return
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 300 && response.StatusCode != 404 {
		log.Printf("go-judge cached file cleanup status: %d", response.StatusCode)
	}
}

func fallbackGoJudgeURL(apiURL string, statusCode int) (string, bool) {
	if statusCode != http.StatusNotFound {
		return "", false
	}
	trimmed := strings.TrimSpace(apiURL)
	if strings.HasSuffix(trimmed, "/run") {
		return strings.TrimSuffix(trimmed, "/run"), true
	}
	return "", false
}

func parseResult(resp CmdResponse, input string) models.TestCaseResult {
	r := models.TestCaseResult{Verdict: "SE", Input: input, Runtime: int64(resp.Time / 1_000_000), MemoryUsage: int64(resp.Memory / 1024), ActualOutput: resp.Files["stdout"]}
	switch resp.Status {
	case "Accepted":
		r.Verdict = "AC"
		r.IsCorrect = resp.ExitStatus == 0
		if !r.IsCorrect {
			r.Verdict = "RE"
			r.ActualOutput = "Runtime Error (Non Zero Exit)"
		}
	case "Time Limit Exceeded", "Memory Limit Exceeded", "Output Limit Exceeded":
		r.Verdict = map[string]string{"Time Limit Exceeded": "TLE", "Memory Limit Exceeded": "MLE", "Output Limit Exceeded": "OLE"}[resp.Status]
		r.ActualOutput = resp.Status
	case "Signalled", "Nonzero Exit Status", "Non Zero Exit Status", "Dangerous Syscall":
		r.Verdict = "RE"
		r.ActualOutput = "Runtime Error: " + resp.Status
	default:
		r.ActualOutput = "Error: " + resp.Status
	}
	return r
}
