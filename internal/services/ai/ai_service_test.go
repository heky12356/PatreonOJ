package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"patreonoj/internal/config"
)

func TestGenerateQuestionTags_DisabledReturnsMockTags(t *testing.T) {
	svc := NewAIService(config.AIConfig{Enabled: false})

	tags, err := svc.GenerateQuestionTags(context.Background(), "题目：两数之和。给定整数数组 nums 和 target，返回两个数下标。")
	if err != nil {
		t.Fatalf("expected mock tags without error, got %v", err)
	}

	assertContainsTag(t, tags, "array")
	assertContainsTag(t, tags, "hash-table")
}

func TestGenerateQuestionTags_ParsesJSONResponse(t *testing.T) {
	svc, cleanup := newTestAIService(t, `{"tags":[" DP ","dp","graph","","Shortest Path"],"confidence":0.91,"evidence":"uses weighted graph transitions"}`)
	defer cleanup()

	tags, err := svc.GenerateQuestionTags(context.Background(), "求带权图最短路径数量")
	if err != nil {
		t.Fatalf("expected parsed tags, got error: %v", err)
	}

	want := []string{"dp", "graph", "shortest-path"}
	if !sameStringSlice(tags, want) {
		t.Fatalf("unexpected tags: got %#v want %#v", tags, want)
	}
}

func TestGenerateQuestionTags_CleansMarkdownCodeFence(t *testing.T) {
	svc, cleanup := newTestAIService(t, "```json\n{\"tags\":[\"string\",\"sliding-window\"],\"confidence\":0.88,\"evidence\":\"longest substring window\"}\n```")
	defer cleanup()

	tags, err := svc.GenerateQuestionTags(context.Background(), "无重复字符的最长子串")
	if err != nil {
		t.Fatalf("expected parsed fenced JSON, got error: %v", err)
	}

	want := []string{"string", "sliding-window"}
	if !sameStringSlice(tags, want) {
		t.Fatalf("unexpected tags: got %#v want %#v", tags, want)
	}
}

func TestGenerateQuestionTags_InvalidJSONReturnsError(t *testing.T) {
	svc, cleanup := newTestAIService(t, "tags: dp, graph")
	defer cleanup()

	if _, err := svc.GenerateQuestionTags(context.Background(), "动态规划"); err == nil {
		t.Fatal("expected invalid JSON error, got nil")
	}
}

func newTestAIService(t *testing.T, content string) (*AIService, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatResponse{
			Choices: []struct {
				Message Message `json:"message"`
			}{
				{Message: Message{Role: "assistant", Content: content}},
			},
		})
	}))

	svc := NewAIService(config.AIConfig{
		Enabled:     true,
		BaseURL:     server.URL,
		Model:       "test-model",
		Temperature: 0.1,
	})
	svc.Client = server.Client()

	return svc, server.Close
}

func assertContainsTag(t *testing.T, tags []string, want string) {
	t.Helper()
	for _, tag := range tags {
		if tag == want {
			return
		}
	}
	t.Fatalf("expected tags %#v to contain %q", tags, want)
}

func sameStringSlice(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
