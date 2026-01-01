package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/models"
	"dachuang/internal/oss"

	"gorm.io/gorm"
)

type JudgeService struct {
	JudgeAPI          string              // 评测系统API地址
	DB                *gorm.DB            // 数据库连接
	HTTPClient        *http.Client        // HTTP客户端
	LocalJudgeService *LocalJudgeService  // 本地评测服务
	Config            *config.JudgeConfig // 评测配置
	OSSClient         *oss.OSS
	OSSBucket         string
}

// NewJudgeService 创建评测服务实例
func NewJudgeService(cfg *config.JudgeConfig, db *gorm.DB, ossClient *oss.OSS, ossBucket string) *JudgeService {
	timeout := time.Duration(cfg.Timeout) * time.Second

	var localJudge *LocalJudgeService
	if cfg.Mode == "local" && cfg.Local.Enabled {
		localJudge = NewLocalJudgeService(&cfg.Local)
	}

	return &JudgeService{
		JudgeAPI:          cfg.APIURL,
		DB:                db,
		Config:            cfg,
		LocalJudgeService: localJudge,
		OSSClient:         ossClient,
		OSSBucket:         ossBucket,
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

	// 5. 保存结果
	// 将测试结果序列化为JSON字符串
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("序列化测试结果失败: %w", err)
	}
	submission.Results = string(resultsJSON)
	submission.Status = "completed"
	if err := js.DB.Save(submission).Error; err != nil {
		return fmt.Errorf("保存评测结果失败: %w", err)
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
	var results []models.TestCaseResult

	// 检测编程语言（简单实现，可以根据代码内容或用户选择来确定）
	language := js.detectLanguage(code)

	log.Printf("Detected language: %s", language)

	// 检查是否支持该语言
	if !js.LocalJudgeService.IsLanguageSupported(language) {
		return nil, fmt.Errorf("不支持的编程语言: %s", language)
	}

	ctx := context.Background()
	for _, tc := range testCases {
		input, expected, err := js.loadTestCaseIO(ctx, tc)
		if err != nil {
			return nil, err
		}

		result, err := js.LocalJudgeService.JudgeCode(code, input, language)
		if err != nil {
			result = &models.TestCaseResult{
				Input:          input,
				ExpectedOutput: expected,
				ActualOutput:   fmt.Sprintf("Error: %v", err),
				IsCorrect:      false,
				Runtime:        0,
				MemoryUsage:    0,
			}
		} else {
			result.Input = input
			result.ExpectedOutput = expected
			result.IsCorrect = strings.TrimSpace(result.ActualOutput) == expected
		}

		results = append(results, *result)
	}

	return results, nil
}

// executeRemoteJudgement 执行远程API评测
func (js *JudgeService) executeRemoteJudgement(code string, testCases []models.TestCase) ([]models.TestCaseResult, error) {
	var results []models.TestCaseResult

	ctx := context.Background()
	for _, tc := range testCases {
		input, expected, err := js.loadTestCaseIO(ctx, tc)
		if err != nil {
			return nil, err
		}

		result := models.TestCaseResult{
			Input:          input,
			ExpectedOutput: expected,
		}

		startTime := time.Now()
		actualOutput, err := js.callJudge(code, input)
		runtime := time.Since(startTime).Milliseconds()

		if err != nil {
			result.ActualOutput = fmt.Sprintf("Error: %v", err)
			result.IsCorrect = false
		} else {
			result.ActualOutput = actualOutput
			result.IsCorrect = strings.TrimSpace(actualOutput) == expected
		}
		result.Runtime = runtime

		results = append(results, result)
	}

	return results, nil
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
