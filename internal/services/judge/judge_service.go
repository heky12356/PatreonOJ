package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"
	assessmentsvc "patreonoj/internal/services/assessment"

	"gorm.io/gorm"
)

type JudgeService struct {
	DB                *gorm.DB            // 数据库连接
	HTTPClient        *http.Client        // HTTP客户端
	LocalJudgeService *LocalJudgeService  // 本地评测服务
	GoJudgeClient     *GoJudgeClient      // Go-Judge 客户端
	Config            *config.JudgeConfig // 评测配置
	OSSClient         *oss.OSS
	OSSBucket         string

	// 图相关的服务和评估服务
	AssessmentService *assessmentsvc.AssessmentService
}

// NewJudgeService 创建评测服务实例
func NewJudgeService(cfg *config.JudgeConfig, db *gorm.DB, ossClient *oss.OSS, ossBucket string,
	assessmentService *assessmentsvc.AssessmentService,
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
		DB:                db,
		Config:            cfg,
		LocalJudgeService: localJudge,
		GoJudgeClient:     goJudgeClient,
		OSSClient:         ossClient,
		OSSBucket:         ossBucket,
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
	// 本地优先模式
	if js.Config.Mode == "local" && js.LocalJudgeService != nil {
		log.Printf("Local judge mode enabled")
		return js.executeLocalJudgement(code, testCases)
	}

	// 远程评测
	results, err := js.executeRemoteJudgement(code, testCases)
	if err == nil {
		return results, nil
	}

	// 远程失败时，如果本地评测可用则自动降级
	if js.LocalJudgeService != nil {
		log.Printf("remote judge failed, fallback to local judge: %v", err)
		return js.executeLocalJudgement(code, testCases)
	}
	return nil, err
}

// loadTestCaseIO 加载测试用例的输入和输出
func (js *JudgeService) loadTestCaseIO(ctx context.Context, tc models.TestCase) (string, string, error) {
	if js.OSSClient == nil || js.OSSBucket == "" {
		return "", "", fmt.Errorf("oss is not configured")
	}
	if strings.TrimSpace(tc.InputKey) == "" || strings.TrimSpace(tc.OutputKey) == "" {
		return "", "", fmt.Errorf("testcase key is empty")
	}

	inputBytes, err := js.OSSClient.GetObjectBytes(ctx, js.OSSBucket, tc.InputKey)
	if err != nil {
		return "", "", fmt.Errorf("read testcase input failed(key=%s): %w", tc.InputKey, err)
	}
	outputBytes, err := js.OSSClient.GetObjectBytes(ctx, js.OSSBucket, tc.OutputKey)
	if err != nil {
		return "", "", fmt.Errorf("read testcase output failed(key=%s): %w", tc.OutputKey, err)
	}

	return string(inputBytes), strings.TrimSpace(string(outputBytes)), nil
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
		return nil, fmt.Errorf("query testcases failed: %w", err)
	}

	if len(testCases) == 0 {
		return nil, fmt.Errorf("question has no available testcases")
	}

	for _, tc := range testCases {
		if strings.TrimSpace(tc.InputKey) == "" || strings.TrimSpace(tc.OutputKey) == "" {
			return nil, fmt.Errorf("testcase record is not oss-based")
		}
	}

	return testCases, nil
}
