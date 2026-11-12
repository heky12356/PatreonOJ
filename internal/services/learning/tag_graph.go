package learning

import (
	"patreonoj/internal/models"
	"sort"
	"strings"
)

type TagSummary struct {
	Name          string `json:"name"`
	QuestionCount int    `json:"question_count"`
}
type TagGraph struct {
	QuestionGraphResult
	Tag              string `json:"tag"`
	TotalNodes       int    `json:"total_nodes"`
	TagQuestionCount int    `json:"tag_question_count"`
	Page             int    `json:"page"`
	PageSize         int    `json:"page_size"`
}

func (s *LearningService) TagCatalog() ([]TagSummary, error) {
	var questions []models.Question
	if err := s.uniqueVisibleQuestions().Select("tags").Find(&questions).Error; err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, q := range questions {
		for _, tag := range splitTags(q.Tags) {
			counts[tag]++
		}
	}
	result := make([]TagSummary, 0, len(counts))
	for tag, n := range counts {
		result = append(result, TagSummary{Name: tag, QuestionCount: n})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].QuestionCount != result[j].QuestionCount {
			return result[i].QuestionCount > result[j].QuestionCount
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (s *LearningService) BuildTagGraph(tag, query string, page, size int) (*TagGraph, error) {
	tag = strings.ToLower(strings.TrimSpace(tag))
	query = strings.ToLower(strings.TrimSpace(query))
	page = max(1, page)
	size = min(120, max(1, size))
	result := &TagGraph{Tag: tag, Page: page, PageSize: size, QuestionGraphResult: QuestionGraphResult{Nodes: []QuestionGraphNode{}, Edges: []QuestionGraphEdge{}}}
	var questions []models.Question
	if err := s.uniqueVisibleQuestions().Select("id, question_id, title, difficulty, tags").Order("id ASC").Find(&questions).Error; err != nil {
		return nil, err
	}
	matched := make([]models.Question, 0)
	for _, q := range questions {
		found := false
		for _, token := range splitTags(q.Tags) {
			if token == tag {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		result.TagQuestionCount++
		if query != "" && !strings.Contains(strings.ToLower(q.Title), query) && !strings.Contains(strings.ToLower(q.QuestionId), query) {
			continue
		}
		matched = append(matched, q)
	}
	result.TotalNodes = len(matched)
	start := min((page-1)*size, len(matched))
	end := min(start+size, len(matched))
	result.Truncated = start > 0 || end < len(matched)
	ids := make([]string, 0, end-start)
	for _, q := range matched[start:end] {
		ids = append(ids, q.QuestionId)
		result.Nodes = append(result.Nodes, QuestionGraphNode{QuestionID: q.QuestionId, QuestionPK: q.Id, Title: q.Title, Difficulty: q.Difficulty, Tags: q.Tags})
	}
	if len(ids) == 0 {
		return result, nil
	}
	var edges []models.QuestionRelationEdge
	if err := s.db.Where("from_question_id IN ? AND to_question_id IN ?", ids, ids).Order("id ASC").Limit(2001).Find(&edges).Error; err != nil {
		return nil, err
	}
	if len(edges) > 2000 {
		result.Truncated = true
		edges = edges[:2000]
	}
	for _, e := range edges {
		if e.EdgeType.Valid() {
			result.Edges = append(result.Edges, QuestionGraphEdge{FromQuestionID: e.FromQuestionID, ToQuestionID: e.ToQuestionID, EdgeType: string(e.EdgeType)})
		}
	}
	return result, nil
}
