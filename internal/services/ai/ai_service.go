package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"patreonoj/internal/config"
)

var ErrAINotImplemented = errors.New("ai implementation is not ready")

// AIService provides AI integration methods.
type AIService struct {
	Config config.AIConfig
	Client *http.Client
}

func NewAIService(cfg config.AIConfig) *AIService {
	return &AIService{
		Config: cfg,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type questionTagAIResponse struct {
	Tags       []string        `json:"tags"`
	Confidence *float64        `json:"confidence"`
	Evidence   json.RawMessage `json:"evidence"`
}

func (s *AIService) callAI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !s.Config.Enabled {
		return "", fmt.Errorf("AI service is disabled")
	}

	reqBody := ChatRequest{
		Model:       s.Config.Model,
		Temperature: s.Config.Temperature,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Config.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
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

// GenerateQuestionTags generates knowledge tags for a question payload. It only
// returns candidate tags and never persists them to question.tags.
func (s *AIService) GenerateQuestionTags(ctx context.Context, questionPayload string) ([]string, error) {
	if s == nil || !s.Config.Enabled {
		return generateMockQuestionTags(questionPayload), nil
	}

	systemPrompt := strings.Join([]string{
		"你是在线评测系统的算法题知识点标注助手。",
		"请只返回 JSON，不要返回 Markdown、解释文本或额外字段。",
		"JSON 格式必须是：{\"tags\":[\"tag\"],\"confidence\":0.0,\"evidence\":\"reason\"}。",
		"tags 使用小写英文短标签，优先从 array, hash-table, string, stack, queue, two-pointers, sliding-window, sorting, binary-search, heap, linked-list, dp, state-transition, linear-dp, string-dp, knapsack, graph, graph-traversal, topological-sort, shortest-path, union-find, mst, tree, dfs, bfs, backtracking, greedy, trie, design 中选择。",
		"tags 数量控制在 2 到 4 个。",
	}, "\n")
	userPrompt := "请为以下算法题生成知识点标签：\n" + strings.TrimSpace(questionPayload)

	content, err := s.callAI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	return parseQuestionTagAIResponse(content)
}

func cleanAIResponse(content string) string {
	content = strings.TrimSpace(content)

	if idx := strings.Index(content, "</think>"); idx != -1 {
		content = content[idx+len("</think>"):]
		content = strings.TrimSpace(content)
	}

	if strings.HasPrefix(content, "```") {
		if newline := strings.IndexAny(content, "\r\n"); newline != -1 {
			content = content[newline+1:]
		} else {
			content = strings.TrimPrefix(content, "```")
		}
	}
	content = strings.TrimSpace(content)
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

func parseQuestionTagAIResponse(content string) ([]string, error) {
	cleaned := cleanAIResponse(content)
	var parsed questionTagAIResponse
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("parse AI tag response as JSON: %w", err)
	}

	tags := normalizeTags(parsed.Tags)
	if len(tags) == 0 {
		return nil, errors.New("AI tag response contains no valid tags")
	}
	return tags, nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = normalizeTag(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.Trim(tag, "`\"'，,;；、")
	tag = strings.ReplaceAll(tag, "\u3000", " ")
	fields := strings.FieldsFunc(tag, func(r rune) bool {
		return unicode.IsSpace(r) || r == '_' || r == '/'
	})
	tag = strings.ToLower(strings.Join(fields, "-"))
	return strings.Trim(tag, "-")
}

func generateMockQuestionTags(questionPayload string) []string {
	text := strings.ToLower(questionPayload)
	text = strings.ReplaceAll(text, "\u3000", " ")

	type rule struct {
		tag      string
		keywords []string
	}
	rules := []rule{
		{tag: "array", keywords: []string{"数组", "序列", "nums", "prices", "区间", "子数组"}},
		{tag: "hash-table", keywords: []string{"哈希", "频率", "two sum", "两数之和", "重复"}},
		{tag: "string", keywords: []string{"字符串", "子串", "word", "括号", "trie"}},
		{tag: "stack", keywords: []string{"栈", "括号", "解码", "单调栈"}},
		{tag: "queue", keywords: []string{"队列", "层序", "bfs"}},
		{tag: "two-pointers", keywords: []string{"双指针", "接雨水", "左右指针"}},
		{tag: "sliding-window", keywords: []string{"滑动窗口", "最长子串", "最小覆盖", "窗口"}},
		{tag: "sorting", keywords: []string{"排序", "合并区间", "sort"}},
		{tag: "binary-search", keywords: []string{"二分", "binary search", "搜索插入", "第 k 大", "最长递增"}},
		{tag: "heap", keywords: []string{"堆", "优先队列", "第 k 大", "top k", "最短路径", "dijkstra"}},
		{tag: "linked-list", keywords: []string{"链表", "linked list"}},
		{tag: "dp", keywords: []string{"动态规划", "dp", "状态转移", "编辑距离", "零钱", "爬楼梯", "打家劫舍", "最长递增", "路径", "最大子数组"}},
		{tag: "state-transition", keywords: []string{"状态转移", "动态规划", "爬楼梯", "打家劫舍", "路径"}},
		{tag: "linear-dp", keywords: []string{"线性 dp", "最大子数组", "打家劫舍", "爬楼梯"}},
		{tag: "string-dp", keywords: []string{"编辑距离", "word1", "word2", "字符串 dp"}},
		{tag: "knapsack", keywords: []string{"背包", "零钱", "硬币"}},
		{tag: "graph", keywords: []string{"图", "课程", "岛屿", "网络", "路径", "城市", "连通", "边", "节点"}},
		{tag: "graph-traversal", keywords: []string{"遍历", "岛屿", "矩阵", "bfs", "dfs", "连通"}},
		{tag: "topological-sort", keywords: []string{"拓扑", "课程表", "依赖"}},
		{tag: "shortest-path", keywords: []string{"最短路", "最短路径", "网络延迟", "dijkstra"}},
		{tag: "union-find", keywords: []string{"并查集", "省份", "连通", "最小生成树"}},
		{tag: "mst", keywords: []string{"最小生成树", "mst", "连接所有点"}},
		{tag: "tree", keywords: []string{"树", "二叉树", "bst", "层序", "祖先", "路径总和"}},
		{tag: "dfs", keywords: []string{"dfs", "深度优先", "回溯", "树", "路径总和"}},
		{tag: "bfs", keywords: []string{"bfs", "广度优先", "层序", "队列"}},
		{tag: "backtracking", keywords: []string{"回溯", "全排列", "子集", "n 皇后", "n皇后"}},
		{tag: "greedy", keywords: []string{"贪心", "跳跃", "加油站", "股票", "任务调度"}},
		{tag: "trie", keywords: []string{"trie", "字典树", "前缀"}},
		{tag: "design", keywords: []string{"设计", "实现", "序列化", "trie"}},
	}

	matches := make([]string, 0, 4)
	for _, r := range rules {
		if containsAny(text, r.keywords) {
			matches = append(matches, r.tag)
		}
	}
	if len(matches) == 0 {
		matches = append(matches, "array", "string")
	}
	matches = normalizeTags(matches)
	if len(matches) > 4 {
		return matches[:4]
	}
	return matches
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
