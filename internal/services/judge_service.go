package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/graph"
	"dachuang/internal/models"
	"dachuang/internal/oss"

	"gorm.io/gorm"
)

type JudgeService struct {
	JudgeAPI          string              // 评测系统API地址
	DB                *gorm.DB            // 数据库连接
	HTTPClient        *http.Client        // HTTP客户端
	LocalJudgeService *LocalJudgeService  // 本地评测服务
	GoJudgeClient     *GoJudgeClient      // Go-Judge 客户端
	Config            *config.JudgeConfig // 评测配置
	OSSClient         *oss.OSS
	OSSBucket         string

	// 图相关的服务和评估服务
	GraphService      *graph.QuestionGraphService
	AssessmentService *AssessmentService
}

// NewJudgeService 创建评测服务实例
func NewJudgeService(cfg *config.JudgeConfig, db *gorm.DB, ossClient *oss.OSS, ossBucket string,
	graphService *graph.QuestionGraphService, assessmentService *AssessmentService,
) *JudgeService {
	timeout := time.Duration(cfg.Timeout) * time.Second

	var localJudge *LocalJudgeService
	if cfg.Mode == "local" && cfg.Local.Enabled {
		localJudge = NewLocalJudgeService(&cfg.Local)
	}

	var goJudgeClient *GoJudgeClient
	if cfg.GoJudge.Enabled {
		goJudgeClient = NewGoJudgeClient(cfg.GoJudge.APIURL, cfg.Timeout)
	}

	return &JudgeService{
		JudgeAPI:          cfg.APIURL,
		DB:                db,
		Config:            cfg,
		LocalJudgeService: localJudge,
		GoJudgeClient:     goJudgeClient,
		OSSClient:         ossClient,
		OSSBucket:         ossBucket,
		GraphService:      graphService,
		AssessmentService: assessmentService,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// JudgeCode 执行代码评测
func (js *JudgeService) JudgeCode(submission *models.Submission) error {
	// 1. 验证提交状态
	if submission.Status == "completed" {
		return fmt.Errorf("提交已完成评测，无需重复评测")
	}

	// 2. 获取测试用例
	testCases, err := js.getTestCases(submission.QuestionID)
	if err != nil {
		return fmt.Errorf("获取测试用例失败: %w", err)
	}

	// 3. 准备评测
	submission.Status = "processing"
	if err = js.DB.Save(submission).Error; err != nil {
		return fmt.Errorf("更新提交状态失败: %w", err)
	}

	log.Print("debug")

	// 4. 执行评测
	results, err := js.executeJudgement(submission.Code, testCases)
	if err != nil {
		return fmt.Errorf("执行评测失败: %w", err)
	}

	var maxRuntime int64
	var maxMem int64
	for _, r := range results {
		if r.Runtime > maxRuntime {
			maxRuntime = r.Runtime
		}
		if r.MemoryUsage > maxMem {
			maxMem = r.MemoryUsage
		}
	}

	if submission.Language == "" {
		submission.Language = js.detectLanguage(submission.Code)
	}
	if submission.CodeLength == 0 {
		submission.CodeLength = len(submission.Code)
	}
	submission.RuntimeMs = maxRuntime
	submission.MemoryKB = maxMem

	// 5. 保存结果
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("序列化测试结果失败: %w", err)
	}
	submission.Results = string(resultsJSON)
	submission.Status = "completed"
	if err := js.DB.Save(submission).Error; err != nil {
		return fmt.Errorf("保存评测结果失败: %w", err)
	}

	// 6. 如果评测通过，触发能力评估更新
	isAc := true
	for _, r := range results {
		if !r.IsCorrect {
			isAc = false
			break
		}
	}

	if isAc {
		if err := js.AssessmentService.UpdateUserMasteryBasedOnResult(submission.UserID, submission.QuestionID, true); err != nil {
			log.Printf("更新用户的掌握度失败: %v", err)
		}
	}

	return nil
}

// executeJudgement 执行实际评测逻辑
func (js *JudgeService) executeJudgement(code string, testCases []models.TestCase) ([]models.TestCaseResult, error) {
	// 根据配置选择评测方式
	if js.Config.Mode == "local" && js.LocalJudgeService != nil {
		// 本地评测
		log.Printf("Local judge mode enabled")

		return js.executeLocalJudgement(code, testCases)
	} else {
		// 远程API评测
		return js.executeRemoteJudgement(code, testCases)
	}
}

// loadTestCaseIO 加载测试用例的输入和输出
func (js *JudgeService) loadTestCaseIO(ctx context.Context, tc models.TestCase) (string, string, error) {
	input := tc.Input
	expected := tc.ExpectedOutput

	if js.OSSClient != nil && js.OSSBucket != "" {
		if tc.InputKey != "" {
			b, err := js.OSSClient.GetObjectBytes(ctx, js.OSSBucket, tc.InputKey)
			if err != nil {
				return "", "", fmt.Errorf("读取测试用例输入失败(key=%s): %w", tc.InputKey, err)
			}
			input = string(b)
		}
		if tc.OutputKey != "" {
			b, err := js.OSSClient.GetObjectBytes(ctx, js.OSSBucket, tc.OutputKey)
			if err != nil {
				return "", "", fmt.Errorf("读取测试用例输出失败(key=%s): %w", tc.OutputKey, err)
			}
			expected = string(b)
		}
	}

	return input, strings.TrimSpace(expected), nil
}

// executeLocalJudgement 执行本地评测
func (js *JudgeService) executeLocalJudgement(code string, testCases []models.TestCase) ([]models.TestCaseResult, error) {
	// 检测编程语言（简单实现，可以根据代码内容或用户选择来确定）
	language := js.detectLanguage(code)
	log.Printf("Detected language: %s", language)

	// 检查是否支持该语言
	if !js.LocalJudgeService.IsLanguageSupported(language) {
		return nil, fmt.Errorf("不支持的编程语言: %s", language)
	}

	inputs := make([]string, 0, len(testCases))
	expectedList := make([]string, 0, len(testCases))
	ctx := context.Background()
	for _, tc := range testCases {
		input, expected, err := js.loadTestCaseIO(ctx, tc)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
		expectedList = append(expectedList, expected)
	}

	batch, err := js.LocalJudgeService.JudgeBatch(code, inputs, language)
	if err != nil {
		return nil, err
	}
	if len(batch) != len(inputs) {
		return nil, fmt.Errorf("本地评测结果数量不匹配: got=%d want=%d", len(batch), len(inputs))
	}

	results := make([]models.TestCaseResult, 0, len(batch))
	for i := range batch {
		r := batch[i]
		r.Input = inputs[i]
		r.ExpectedOutput = normalizeOutput(expectedList[i])
		r.IsCorrect = strings.TrimSpace(normalizeOutput(r.ActualOutput)) == r.ExpectedOutput
		results = append(results, r)
	}

	return results, nil
}

// normalizeOutput 标准化输出格式
func normalizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)

	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

// executeRemoteJudgement 执行远程API评测 (Go-Judge)
func (js *JudgeService) executeRemoteJudgement(code string, testCases []models.TestCase) ([]models.TestCaseResult, error) {
	if js.GoJudgeClient == nil {
		return nil, fmt.Errorf("go-judge client is not initialized")
	}

	// 1. 准备输入数据
	inputs := make([]string, 0, len(testCases))
	expectedList := make([]string, 0, len(testCases))
	ctx := context.Background()

	for _, tc := range testCases {
		input, expected, err := js.loadTestCaseIO(ctx, tc)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
		expectedList = append(expectedList, expected)
	}

	// 2. 检测语言
	language := js.detectLanguage(code)

	// 3. 调用 Go-Judge (批量执行)
	memLimit := int64(js.Config.GoJudge.MaxMemory)
	if memLimit == 0 {
		memLimit = 256
	}
	timeLimit := int64(js.Config.GoJudge.MaxTime)
	if timeLimit == 0 {
		timeLimit = 5000
	}

	results, err := js.GoJudgeClient.Run(code, language, inputs, timeLimit, memLimit)
	if err != nil {
		return nil, fmt.Errorf("go-judge execution failed: %w", err)
	}

	// 4. 比对结果
	for i := range results {
		// GoJudgeClient 已经填好了 Runtime, Memory, ActualOutput (maybe error message)
		// 我们需要比对 ExpectedOutput
		if results[i].ActualOutput == "Time Limit Exceeded" ||
			results[i].ActualOutput == "Memory Limit Exceeded" ||
			strings.HasPrefix(results[i].ActualOutput, "Runtime Error") ||
			strings.HasPrefix(results[i].ActualOutput, "Compile Error") ||
			strings.HasPrefix(results[i].ActualOutput, "Error") {
			results[i].IsCorrect = false
		} else {
			results[i].ExpectedOutput = normalizeOutput(expectedList[i])
			results[i].ActualOutput = strings.TrimSpace(results[i].ActualOutput)
			// 标准化 Output
			results[i].IsCorrect = strings.TrimSpace(normalizeOutput(results[i].ActualOutput)) == results[i].ExpectedOutput
		}
	}

	return results, nil
}

func (js *JudgeService) DetectLanguage(code string) string {
	return js.detectLanguage(code)
}

// detectLanguage 检测编程语言（简单实现）
func (js *JudgeService) detectLanguage(code string) string {
	// 简单的语言检测逻辑，实际项目中可以更复杂
	if strings.Contains(code, "package main") || strings.Contains(code, "func main()") {
		return "go"
	} else if strings.Contains(code, "def ") || strings.Contains(code, "import ") {
		return "python"
	} else if strings.Contains(code, "#include") || strings.Contains(code, "int main()") {
		return "cpp"
	} else if strings.Contains(code, "public class") || strings.Contains(code, "public static void main") {
		return "java"
	}

	// 默认返回Go语言
	return "go"
}

// getTestCases 获取题目测试用例
func (js *JudgeService) getTestCases(questionID int) ([]models.TestCase, error) {
	var testCases []models.TestCase

	if err := js.DB.Where("question_id = ? AND is_hidden = ?", questionID, false).
		Find(&testCases).Error; err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	if len(testCases) != 0 {
		return testCases, nil
	}

	if js.OSSClient == nil || js.OSSBucket == "" {
		return nil, fmt.Errorf("题目没有可用的测试用例")
	}

	var question models.Question
	if err := js.DB.Where("id = ?", questionID).First(&question).Error; err != nil {
		return nil, fmt.Errorf("题目没有可用的测试用例")
	}

	prefix := fmt.Sprintf("problems/%d/", question.QuestionNumber)

	ctx := context.Background()
	objects, err := js.OSSClient.ListObjects(ctx, js.OSSBucket, prefix, true)
	if err != nil {
		return nil, fmt.Errorf("获取测试用例失败: %w", err)
	}

	type pair struct {
		inKey  string
		outKey string
	}
	pairs := make(map[int]*pair)

	for _, key := range objects {
		if strings.HasSuffix(key, "/") {
			continue
		}

		name := path.Base(key)
		if strings.HasSuffix(name, ".in") {
			n, err := strconv.Atoi(strings.TrimSuffix(name, ".in"))
			if err != nil {
				continue
			}
			p := pairs[n]
			if p == nil {
				p = &pair{}
				pairs[n] = p
			}
			p.inKey = key
			continue
		}
		if strings.HasSuffix(name, ".out") {
			n, err := strconv.Atoi(strings.TrimSuffix(name, ".out"))
			if err != nil {
				continue
			}
			p := pairs[n]
			if p == nil {
				p = &pair{}
				pairs[n] = p
			}
			p.outKey = key
			continue
		}
	}

	nums := make([]int, 0, len(pairs))
	for n := range pairs {
		nums = append(nums, n)
	}
	sort.Ints(nums)

	for _, n := range nums {
		p := pairs[n]
		if p == nil || p.inKey == "" || p.outKey == "" {
			continue
		}
		testCases = append(testCases, models.TestCase{
			QuestionID: questionID,
			InputKey:   p.inKey,
			OutputKey:  p.outKey,
			IsHidden:   false,
		})
	}

	if len(testCases) == 0 {
		return nil, fmt.Errorf("题目没有可用的测试用例")
	}

	return testCases, nil
}

// callJudge 调用评测系统API
func (js *JudgeService) callJudge(code, input string) (string, error) {
	requestBody := map[string]interface{}{
		"code":    code,
		"input":   input,
		"timeout": 5000, // 5秒超时
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %w", err)
	}

	req, err := http.NewRequest("POST", js.JudgeAPI, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := js.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求评测系统失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("评测系统返回错误状态码: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	var response struct {
		Output     string `json:"output"`
		Error      string `json:"error"`
		TimeUsed   int64  `json:"time_used"`
		MemoryUsed int64  `json:"memory_used"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析JSON响应失败: %w", err)
	}

	if response.Error != "" {
		return "", fmt.Errorf("评测系统错误: %s", response.Error)
	}

	return response.Output, nil
}
