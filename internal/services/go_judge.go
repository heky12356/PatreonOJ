package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"dachuang/internal/models"
)

// GoJudgeClient 负责与 go-judge 服务通信
type GoJudgeClient struct {
	APIURL     string
	HTTPClient *http.Client
}

// NewGoJudgeClient 创建新的 go-judge 客户端
func NewGoJudgeClient(apiURL string, timeout int) *GoJudgeClient {
	return &GoJudgeClient{
		APIURL: apiURL,
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// CmdRequest go-judge 请求结构 (部分字段)
type CmdRequest struct {
	Args          []string           `json:"args"`
	Env           []string           `json:"env,omitempty"`
	Files         []*CmdFile         `json:"files,omitempty"`
	CPULimit      uint64             `json:"cpuLimit,omitempty"`    // ns
	ClockLimit    uint64             `json:"clockLimit,omitempty"`  // ns
	MemoryLimit   uint64             `json:"memoryLimit,omitempty"` // byte
	ProcLimit     uint64             `json:"procLimit,omitempty"`
	StackLimit    uint64             `json:"stackLimit,omitempty"` // byte
	CopyIn        map[string]CmdFile `json:"copyIn,omitempty"`
	CopyOut       []string           `json:"copyOut,omitempty"`
	CopyOutCached []string           `json:"copyOutCached,omitempty"`
	CopyOutDir    string             `json:"copyOutDir,omitempty"`
}

// CmdFile 文件定义
type CmdFile struct {
	Name    string  `json:"name,omitempty"`
	Content *string `json:"content,omitempty"`
	Src     *string `json:"src,omitempty"`
	FileID  *string `json:"fileId,omitempty"`
	Max     int64   `json:"max,omitempty"` // max size for output
}

// CmdResponse go-judge 响应结构
type CmdResponse struct {
	Status     string            `json:"status"`
	ExitStatus int               `json:"exitStatus"`
	Error      string            `json:"error"`
	Time       uint64            `json:"time"`   // ns
	Memory     uint64            `json:"memory"` // byte
	RunTime    uint64            `json:"runTime"`
	Files      map[string]string `json:"files"`
	FileIds    map[string]string `json:"fileIds"`
}

// Run 执行判定
func (c *GoJudgeClient) Run(code string, language string, inputs []string, timeLimitMs int64, memoryLimitMB int64) ([]models.TestCaseResult, error) {
	// 转换限制单位
	cpuLimitNs := uint64(timeLimitMs) * 1_000_000
	clockLimitNs := cpuLimitNs * 3 // 给多一点墙上时间，防止IO等导致超时
	memoryLimitByte := uint64(memoryLimitMB) * 1024 * 1024

	// 针对不同语言的策略：
	switch language {
	case "cpp", "go":
		// 需要编译的语言
		return c.runCompiledLanguage(code, language, inputs, cpuLimitNs, clockLimitNs, memoryLimitByte)
	case "python":
		// 解释型语言
		return c.runInterpretedLanguage(code, language, inputs, cpuLimitNs, clockLimitNs, memoryLimitByte)
	case "java":
		return c.runJava(code, inputs, cpuLimitNs, clockLimitNs, memoryLimitByte)
	default:
		return nil, fmt.Errorf("unsupported language for go-judge: %s", language)
	}
}

// runCompiledLanguage 处理编译型语言 (C++, Go)
func (c *GoJudgeClient) runCompiledLanguage(code, language string, inputs []string, cpuLimit, clockLimit, memoryLimit uint64) ([]models.TestCaseResult, error) {
	// 步骤 1: 编译
	// 构造编译请求
	var compileCmd CmdRequest
	var srcName, exeName string
	defaultEnv := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	if language == "cpp" {
		srcName = "main.cpp"
		exeName = "main"
		compileCmd = CmdRequest{
			Args: []string{"g++", srcName, "-o", exeName, "-O2", "-std=c++17"}, // 假设环境有 g++
			Env:  []string{defaultEnv},
			Files: []*CmdFile{
				{Content: new(string)},
				{Name: "stdout", Max: 10240},
				{Name: "stderr", Max: 10240},
			},
			CopyIn: map[string]CmdFile{
				srcName: {Content: &code},
			},
			CopyOut:     []string{exeName},       // 保留可执行文件供后续使用
			CPULimit:    10 * 1000 * 1000 * 1000, // 10s 编译时间
			ClockLimit:  10 * 1000 * 1000 * 1000,
			MemoryLimit: 512 * 1024 * 1024,
			ProcLimit:   50,
		}
	} else if language == "go" {
		srcName = "main.go"
		exeName = "main"
		compileCmd = CmdRequest{
			Args: []string{"go", "build", "-o", exeName, srcName},
			Env:  []string{"GOCACHE=/tmp", "GOMODCACHE=/tmp", defaultEnv}, // Go build env
			Files: []*CmdFile{
				{Content: new(string)},
				{Name: "stdout", Max: 10240},
				{Name: "stderr", Max: 10240},
			},
			CopyIn: map[string]CmdFile{
				srcName: {Content: &code},
			},
			CopyOut:     []string{exeName},
			CPULimit:    10 * 1000 * 1000 * 1000,
			ClockLimit:  10 * 1000 * 1000 * 1000,
			MemoryLimit: 512 * 1024 * 1024,
			ProcLimit:   50,
		}
	}

	// 发送编译请求
	compileReqBody := map[string]interface{}{
		"cmd": []CmdRequest{compileCmd},
	}
	compileResps, err := c.doRequest(compileReqBody)
	if err != nil {
		return nil, fmt.Errorf("compile request failed: %w", err)
	}

	compileResult := compileResps[0]
	if compileResult.Status != "Accepted" {
		return nil, fmt.Errorf("Compile Error: %s %s", compileResult.Status, compileResult.Error)
	}

	// 修正编译请求，使用 copyOutCached
	compileCmd.CopyOut = nil
	compileCmd.CopyOutCached = []string{exeName}

	// 重新构造请求体
	compileReqBody = map[string]interface{}{
		"cmd": []CmdRequest{compileCmd},
	}
	compileResps, err = c.doRequest(compileReqBody)
	if err != nil {
		return nil, err
	}

	if compileResps[0].Status != "Accepted" {
		// 错误处理... 简化
		return getAllErrorResult(inputs, "Compile Error"), nil
	}

	exeFileId := compileResps[0].FileIds[exeName]
	if exeFileId == "" {
		return nil, fmt.Errorf("compile success but no executable fileId returned")
	}

	// 步骤 2: 运行
	// 构造批量运行请求
	var runCmds []CmdRequest
	for _, input := range inputs {
		inputContent := input
		runCmd := CmdRequest{
			Args: []string{"./" + exeName},
			Env:  []string{defaultEnv},
			Files: []*CmdFile{
				{Content: &inputContent},     // stdin
				{Name: "stdout", Max: 10240}, // stdout (limit 10KB)
				{Name: "stderr", Max: 10240}, // stderr
			},
			CopyIn: map[string]CmdFile{
				exeName: {FileID: &exeFileId}, // 使用之前的 fileId
			},
			CPULimit:    cpuLimit,
			ClockLimit:  clockLimit,
			MemoryLimit: memoryLimit,
			ProcLimit:   50,
		}
		runCmds = append(runCmds, runCmd)
	}

	runReqBody := map[string]interface{}{
		"cmd": runCmds,
	}
	runResps, err := c.doRequest(runReqBody)
	if err != nil {
		return nil, err
	}

	// 转换结果
	results := make([]models.TestCaseResult, len(inputs))
	for i, resp := range runResps {
		results[i] = parseResult(resp, inputs[i])
	}

	return results, nil
}

// runInterpretedLanguage 处理解释型语言 (Python)
func (c *GoJudgeClient) runInterpretedLanguage(code, language string, inputs []string, cpuLimit, clockLimit, memoryLimit uint64) ([]models.TestCaseResult, error) {
	defaultEnv := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	var runCmds []CmdRequest
	for _, input := range inputs {
		inputContent := input
		codeRef := code // copy locally
		runCmd := CmdRequest{
			Args: []string{"python3", "main.py"},
			Env:  []string{defaultEnv, "PYTHONIOENCODING=utf-8"}, // Add encoding for safety
			Files: []*CmdFile{
				{Content: &inputContent},
				{Name: "stdout", Max: 10240},
				{Name: "stderr", Max: 10240},
			},
			CopyIn: map[string]CmdFile{
				"main.py": {Content: &codeRef},
			},
			CPULimit:    cpuLimit,
			ClockLimit:  clockLimit,
			MemoryLimit: memoryLimit,
			ProcLimit:   50,
		}
		runCmds = append(runCmds, runCmd)
	}

	runReqBody := map[string]interface{}{
		"cmd": runCmds,
	}
	runResps, err := c.doRequest(runReqBody)
	if err != nil {
		return nil, err
	}

	results := make([]models.TestCaseResult, len(inputs))
	for i, resp := range runResps {
		results[i] = parseResult(resp, inputs[i])
	}
	return results, nil
}

// runJava 处理 Java (需要编译)
func (c *GoJudgeClient) runJava(code string, inputs []string, cpuLimit, clockLimit, memoryLimit uint64) ([]models.TestCaseResult, error) {
	// 1. Compile Main.java -> Main.class
	defaultEnv := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	compileCmd := CmdRequest{
		Args: []string{"javac", "Main.java"},
		Env:  []string{defaultEnv},
		Files: []*CmdFile{
			{Content: new(string)},
			{Name: "stdout", Max: 10240},
			{Name: "stderr", Max: 10240},
		},
		CopyIn: map[string]CmdFile{
			"Main.java": {Content: &code},
		},
		CopyOutCached: []string{"Main.class"},
		CPULimit:      10 * 1000 * 1000 * 1000,
		ClockLimit:    10 * 1000 * 1000 * 1000,
		MemoryLimit:   512 * 1024 * 1024,
		ProcLimit:     50,
	}

	compileReqBody := map[string]interface{}{"cmd": []CmdRequest{compileCmd}}
	compileResps, err := c.doRequest(compileReqBody)
	if err != nil {
		return nil, err
	}

	if compileResps[0].Status != "Accepted" {
		return getAllErrorResult(inputs, "Compile Error"), nil
	}

	classFileId := compileResps[0].FileIds["Main.class"]
	if classFileId == "" {
		return nil, fmt.Errorf("java compile success but no class fileId")
	}

	// 2. Run java
	var runCmds []CmdRequest
	for _, input := range inputs {
		inputContent := input
		runCmd := CmdRequest{
			Args: []string{"java", "Main"}, // 假设 CLASSPATH 默认包含 .
			Env:  []string{defaultEnv},
			Files: []*CmdFile{
				{Content: &inputContent},
				{Name: "stdout", Max: 10240},
				{Name: "stderr", Max: 10240},
			},
			CopyIn: map[string]CmdFile{
				"Main.class": {FileID: &classFileId},
			},
			CPULimit:    cpuLimit,
			ClockLimit:  clockLimit,
			MemoryLimit: memoryLimit,
			ProcLimit:   50,
		}
		runCmds = append(runCmds, runCmd)
	}

	runReqBody := map[string]interface{}{"cmd": runCmds}
	runResps, err := c.doRequest(runReqBody)
	if err != nil {
		return nil, err
	}

	results := make([]models.TestCaseResult, len(inputs))
	for i, resp := range runResps {
		results[i] = parseResult(resp, inputs[i])
	}
	return results, nil
}

func (c *GoJudgeClient) doRequest(body interface{}) ([]CmdResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.APIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("go-judge api error: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	// go-judge 返回的是 Response 数组
	var results []CmdResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	return results, nil
}

func parseResult(resp CmdResponse, input string) models.TestCaseResult {
	r := models.TestCaseResult{
		Input:       input,
		Runtime:     int64(resp.Time / 1_000_000), // ns -> ms
		MemoryUsage: int64(resp.Memory / 1024),    // byte -> KB
	}

	// 获取 stdout
	if resp.Files != nil {
		r.ActualOutput = resp.Files["stdout"]
	}

	// Status Mappings
	switch resp.Status {
	case "Accepted":
	case "Time Limit Exceeded":
		r.ActualOutput = "Time Limit Exceeded"
	case "Memory Limit Exceeded":
		r.ActualOutput = "Memory Limit Exceeded"
	case "Signalled":
		r.ActualOutput = fmt.Sprintf("Runtime Error (Signal %s)", resp.Error)
	case "Non Zero Exit Status":
		r.ActualOutput = "Runtime Error (Non Zero Exit)"
	default:
		r.ActualOutput = fmt.Sprintf("Error: %s", resp.Status)
	}

	return r
}

func getAllErrorResult(inputs []string, msg string) []models.TestCaseResult {
	res := make([]models.TestCaseResult, len(inputs))
	for i := range inputs {
		res[i] = models.TestCaseResult{
			Input:        inputs[i],
			ActualOutput: msg,
			IsCorrect:    false,
		}
	}
	return res
}
