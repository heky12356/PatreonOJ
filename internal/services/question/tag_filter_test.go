package question

import (
	"patreonoj/internal/models"
	"testing"
)

func TestExactTagFilterCombinesVisibilityDifficultySearchAndPagination(t *testing.T) {
	db := newQuestionServiceTestDB(t)
	rows := []models.Question{
		{QuestionId: "A", Title: "target", Tags: "数组,二分查找", Difficulty: "简单", Status: "published"},
		{QuestionId: "B", Title: "target", Tags: "二维数组", Difficulty: "简单", Status: "published"},
		{QuestionId: "C", Title: "target", Tags: " 数组；哈希表 ", Difficulty: "困难", Status: "published"},
		{QuestionId: "D", Title: "other", Tags: "数组|树", Difficulty: "简单", Status: "published"},
		{QuestionId: "E", Title: "target 2", Tags: "数组，前缀和", Difficulty: "简单", Status: "published"},
		{QuestionId: "F", Title: "target draft", Tags: "数组", Difficulty: "简单", Status: "draft"},
		{QuestionId: "G", Title: "English", Tags: " Array\\hash ", Difficulty: "简单", Status: "published"},
		{QuestionId: "H", Title: "literal", Tags: "literal%_tag", Status: "published"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewQuestionService(db)
	for page, want := range map[int]string{1: "A", 2: "E"} {
		result, err := svc.ListFiltered("target", "简单", " 数组 ", false, page, 1)
		if err != nil {
			t.Fatal(err)
		}
		if result.TotalCnt != 2 || len(result.Result) != 1 || result.Result[0].QuestionId != want {
			t.Fatalf("page %d: %+v", page, result)
		}
	}
	result, err := svc.ListFiltered("", "", "数组", false, 1, 20)
	if err != nil || result.TotalCnt != 4 {
		t.Fatalf("public: %+v %v", result, err)
	}
	result, err = svc.ListFiltered("target", "简单", "数组", true, 1, 20)
	if err != nil || result.TotalCnt != 3 {
		t.Fatalf("admin: %+v %v", result, err)
	}
	result, err = svc.ListFiltered("", "", "ARRAY", false, 1, 20)
	if err != nil || result.TotalCnt != 1 || result.Result[0].QuestionId != "G" {
		t.Fatalf("case: %+v %v", result, err)
	}
	for _, tag := range []string{"%", "_", "unknown", "' OR 1=1 --"} {
		result, err = svc.ListFiltered("", "", tag, false, 1, 20)
		if err != nil || result.TotalCnt != 0 {
			t.Fatalf("literal %q: %+v %v", tag, result, err)
		}
	}
	result, err = svc.ListFiltered("", "", "literal%_tag", false, 1, 20)
	if err != nil || result.TotalCnt != 1 {
		t.Fatalf("literal tag: %+v %v", result, err)
	}
}
