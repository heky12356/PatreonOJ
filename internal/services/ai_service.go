package services

import (
	"bytes"
	"context"
	"dachuang/internal/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AIService AI服务
type AIService struct {
	Config config.AIConfig
	Client *http.Client
}

// NewAIService 创建AI服务
func NewAIService(cfg config.AIConfig) *AIService {
	return &AIService{
		Config: cfg,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ChatRequest OpenAI Chat Completion Request
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

// Message Chat Message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse OpenAI Chat Completion Response
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// QuestionRelation 题目分析结果
type QuestionRelation struct {
	SourceID     int    `json:"source_id"`
	TargetID     int    `json:"target_id"`
	RelationType string `json:"relation_type"` // PREREQUISITE, SIMILAR
	Reason       string `json:"reason"`
}

// SkillRelation 技能分析结果
type SkillRelation struct {
	ParentSkill string `json:"parent_skill"`
	ChildSkill  string `json:"child_skill"`
	Reason      string `json:"reason"`
}

// callAI 通用AI调用方法
func (s *AIService) callAI(ctx context.Context, systemPrompt, userPropmt string) (string, error) {
	if !s.Config.Enabled {
		return "", fmt.Errorf("AI service is disabled")
	}

	reqBody := ChatRequest{
		Model:       s.Config.Model,
		Temperature: s.Config.Temperature,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPropmt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.Config.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if s.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.Config.APIKey)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from AI")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// AnalyzeQuestionRelations 分析题目间的关系
// newQuestion: 新题目内容 (JSON string or description)
// candidateQuestions: 候选题目列表 (JSON string with IDs and descriptions)
func (s *AIService) AnalyzeQuestionRelations(ctx context.Context, newQuestion string, candidateQuestions string) ([]QuestionRelation, error) {
	systemPrompt := `
你是一个算法题目专家。你的任务是分析一道新题目与现有题目库中的题目之间的关系。
主要关注两种关系：
1. **PREREQUISITE (前置)**: 如果理解或解决"新题目"需要先掌握"现有题目"中的知识点（通常是因为现有题目更基础），则"现有题目"是"新题目"的前置。
2. **SIMILAR (相似)**: 如果两道题考察的核心算法、数据结构或解题思路非常相似。

请返回 JSON 格式的数组，不包含任何 Markdown 格式。
格式示例:
[
  {"source_id": <现有题目ID>, "target_id": <新题目ID (通常用0表示或由用户指定)>, "relation_type": "PREREQUISITE", "reason": "需要先掌握二分查找"},
  {"source_id": <现有题目ID>, "target_id": <新题目ID>, "relation_type": "SIMILAR", "reason": "都是动态规划背包问题"}
]
`
	userPrompt := fmt.Sprintf("新题目:\n%s\n\n候选现有题目列表:\n%s", newQuestion, candidateQuestions)

	respContent, err := s.callAI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// 清理可能的 Markdown 标记
	respContent = strings.TrimSpace(respContent)
	respContent = strings.TrimPrefix(respContent, "```json")
	respContent = strings.TrimPrefix(respContent, "```")
	respContent = strings.TrimSuffix(respContent, "```")

	var relations []QuestionRelation
	if err := json.Unmarshal([]byte(respContent), &relations); err != nil {
		return nil, fmt.Errorf("parse error: %w, content: %s", err, respContent)
	}

	return relations, nil
}

// AnalyzeSkillTree 构建技能树关系
// skills: 所有技能标签列表
func (s *AIService) AnalyzeSkillTree(ctx context.Context, skills []string) ([]SkillRelation, error) {
	systemPrompt := `
你是一个计算机科学教育专家。给定一组编程算法领域的技能标签，请构建一个“技能树”结构。
你需要找出技能之间的**直接依赖关系**。
规则：
1. 如果学习 Skill B 通常需要先掌握 Skill A，则 A 是 B 的父节点 (Parent)。
2. 只构建直接的父子关系，不要跳级（例如 A->B->C，不要输出 A->C）。
3. 如果两个技能是同级或无依赖，不要输出。

请返回 JSON 格式的数组，不包含 Markdown。
格式示例:
[
  {"parent_skill": "Sorting", "child_skill": "Merge Sort", "reason": "归并排序是排序算法的一种"},
  {"parent_skill": "Graph Theory", "child_skill": "Shortest Path", "reason": "最短路是图论中的问题"}
]
`
	skillsJSON, _ := json.Marshal(skills)
	userPrompt := fmt.Sprintf("技能列表: %s", string(skillsJSON))

	respContent, err := s.callAI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	respContent = strings.TrimSpace(respContent)
	respContent = strings.TrimPrefix(respContent, "```json")
	respContent = strings.TrimPrefix(respContent, "```")
	respContent = strings.TrimSuffix(respContent, "```")

	var relations []SkillRelation
	if err := json.Unmarshal([]byte(respContent), &relations); err != nil {
		return nil, fmt.Errorf("parse error: %w, content: %s", err, respContent)
	}

	return relations, nil
}
