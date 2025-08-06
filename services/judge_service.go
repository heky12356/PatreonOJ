package services

import (
    "bytes"
    "dachuang/models"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "time"
    "gorm.io/gorm"
)

type JudgeService struct {
    JudgeAPI    string   // 评测系统API地址
    DB          *gorm.DB // 数据库连接
    HTTPClient  *http.Client // HTTP客户端
}

// NewJudgeService 创建评测服务实例
func NewJudgeService(api string, db *gorm.DB) *JudgeService {
    return &JudgeService{
        JudgeAPI: api,
        DB:       db,
        HTTPClient: &http.Client{
            Timeout: 15 * time.Second, // 增加超时时间
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
    if err := js.DB.Save(submission).Error; err != nil {
        return fmt.Errorf("更新提交状态失败: %w", err)
    }

    // 4. 执行评测
    results, err := js.executeJudgement(submission.Code, testCases)
    if err != nil {
        return fmt.Errorf("执行评测失败: %w", err)
    }

    // 5. 保存结果
    submission.Results = results
    submission.Status = "completed"
    if err := js.DB.Save(submission).Error; err != nil {
        return fmt.Errorf("保存评测结果失败: %w", err)
    }

    return nil
}

// executeJudgement 执行实际评测逻辑
func (js *JudgeService) executeJudgement(code string, testCases []models.TestCase) ([]models.TestCaseResult, error) {
    var results []models.TestCaseResult

    for _, tc := range testCases {
        result := models.TestCaseResult{
            Input:          tc.Input,
            ExpectedOutput: tc.ExpectedOutput,
        }

        // 调用评测系统
        startTime := time.Now()
        actualOutput, err := js.callJudge(code, tc.Input)
        runtime := time.Since(startTime).Milliseconds()

        if err != nil {
            result.ActualOutput = fmt.Sprintf("Error: %v", err)
            result.IsCorrect = false
        } else {
            result.ActualOutput = actualOutput
            result.IsCorrect = actualOutput == tc.ExpectedOutput
        }
        result.Runtime = runtime

        results = append(results, result)
    }

    return results, nil
}

// getTestCases 获取题目测试用例
func (js *JudgeService) getTestCases(questionID string) ([]models.TestCase, error) {
    var testCases []models.TestCase

    if err := js.DB.Where("question_id = ? AND is_hidden = ?", questionID, false).
        Find(&testCases).Error; err != nil {
        return nil, fmt.Errorf("数据库查询失败: %w", err)
    }

    if len(testCases) == 0 {
        return nil, fmt.Errorf("题目ID %s 没有可用的测试用例", questionID)
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