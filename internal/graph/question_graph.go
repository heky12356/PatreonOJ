package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// QuestionGraphService 题目图服务
type QuestionGraphService struct {
	client *Neo4jClient
}

// NewQuestionGraphService 创建新的题目图服务
func NewQuestionGraphService(client *Neo4jClient) *QuestionGraphService {
	return &QuestionGraphService{
		client: client,
	}
}

// CreateOrUpdateQuestion 创建或更新题目节点
func (s *QuestionGraphService) CreateOrUpdateQuestion(ctx context.Context, question QuestionNode) error {
	query := `
		MERGE (q:Question {question_number: $question_number})
		ON CREATE SET 
			q.title = $title,
			q.difficulty = $difficulty,
			q.tags = $tags,
			q.status = $status,
			q.created_at = $created_at,
			q.updated_at = $updated_at
		ON MATCH SET 
			q.title = $title,
			q.difficulty = $difficulty,
			q.tags = $tags,
			q.status = $status,
			q.updated_at = $updated_at
	`

	params := map[string]interface{}{
		"question_number": question.QuestionNumber,
		"title":          question.Title,
		"difficulty":     question.Difficulty,
		"tags":           question.Tags,
		"status":         question.Status,
		"created_at":     question.CreatedAt,
		"updated_at":     question.UpdatedAt,
	}

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// DeleteQuestion 删除题目节点及其所有关系
func (s *QuestionGraphService) DeleteQuestion(ctx context.Context, questionNumber int) error {
	query := `
		MATCH (q:Question {question_number: $question_number})
		DETACH DELETE q
	`

	params := map[string]interface{}{
		"question_number": questionNumber,
	}

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// CreateRelation 创建题目之间的关系
func (s *QuestionGraphService) CreateRelation(ctx context.Context, relation QuestionRelation) error {
	query := fmt.Sprintf(`
		MATCH (from:Question {question_number: $from_question_number})
		MATCH (to:Question {question_number: $to_question_number})
		MERGE (from)-[r:%s]->(to)
		SET r.weight = $weight,
			r.description = $description,
			r.created_at = $created_at
	`, relation.RelationType)

	params := map[string]interface{}{
		"from_question_number": relation.FromQuestionNumber,
		"to_question_number":   relation.ToQuestionNumber,
		"weight":              relation.Weight,
		"description":         relation.Description,
		"created_at":          relation.CreatedAt,
	}

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// DeleteRelation 删除题目之间的关系
func (s *QuestionGraphService) DeleteRelation(ctx context.Context, fromQuestion, toQuestion int, relationType RelationshipType) error {
	query := fmt.Sprintf(`
		MATCH (from:Question {question_number: $from_question_number})-[r:%s]->(to:Question {question_number: $to_question_number})
		DELETE r
	`, relationType)

	params := map[string]interface{}{
		"from_question_number": fromQuestion,
		"to_question_number":   toQuestion,
	}

	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// GetPrerequisites 获取题目的前置题目
func (s *QuestionGraphService) GetPrerequisites(ctx context.Context, questionNumber int) ([]QuestionNode, error) {
	query := `
		MATCH (pre:Question)-[:PREREQUISITE]->(q:Question {question_number: $question_number})
		RETURN pre.question_number as question_number, pre.title as title, pre.difficulty as difficulty, 
			   pre.tags as tags, pre.status as status, pre.created_at as created_at, pre.updated_at as updated_at
		ORDER BY pre.question_number
	`

	params := map[string]interface{}{
		"question_number": questionNumber,
	}

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var questions []QuestionNode
		for result.Next(ctx) {
			record := result.Record()
			
			// 安全地处理时间字段
			var createdAt, updatedAt time.Time
			if record.Values[5] != nil {
				if t, ok := record.Values[5].(time.Time); ok {
					createdAt = t
				}
			}
			if record.Values[6] != nil {
				if t, ok := record.Values[6].(time.Time); ok {
					updatedAt = t
				}
			}
			
			question := QuestionNode{
				QuestionNumber: int(record.Values[0].(int64)),
				Title:          record.Values[1].(string),
				Difficulty:     record.Values[2].(string),
				Tags:           record.Values[3].(string),
				Status:         record.Values[4].(string),
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
			}
			questions = append(questions, question)
		}

		return questions, result.Err()
	})

	if err != nil {
		return nil, err
	}

	return result.([]QuestionNode), nil
}

// GetNextLevelQuestions 获取可以进阶到的题目
func (s *QuestionGraphService) GetNextLevelQuestions(ctx context.Context, questionNumber int) ([]QuestionNode, error) {
	query := `
		MATCH (q:Question {question_number: $question_number})-[:NEXT_LEVEL]->(next:Question)
		RETURN next.question_number as question_number, next.title as title, next.difficulty as difficulty,
			   next.tags as tags, next.status as status, next.created_at as created_at, next.updated_at as updated_at
		ORDER BY next.question_number
	`

	params := map[string]interface{}{
		"question_number": questionNumber,
	}

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var questions []QuestionNode
		for result.Next(ctx) {
			record := result.Record()
			
			// 安全地处理时间字段
			var createdAt, updatedAt time.Time
			if record.Values[5] != nil {
				if t, ok := record.Values[5].(time.Time); ok {
					createdAt = t
				}
			}
			if record.Values[6] != nil {
				if t, ok := record.Values[6].(time.Time); ok {
					updatedAt = t
				}
			}
			
			question := QuestionNode{
				QuestionNumber: int(record.Values[0].(int64)),
				Title:          record.Values[1].(string),
				Difficulty:     record.Values[2].(string),
				Tags:           record.Values[3].(string),
				Status:         record.Values[4].(string),
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
			}
			questions = append(questions, question)
		}

		return questions, result.Err()
	})

	if err != nil {
		return nil, err
	}

	return result.([]QuestionNode), nil
}

// FindLearningPath 查找从起始题目到目标题目的学习路径
func (s *QuestionGraphService) FindLearningPath(ctx context.Context, startQuestion, endQuestion int) (*LearningPath, error) {
	query := `
		MATCH path = shortestPath((start:Question {question_number: $start_question})-[:PREREQUISITE|NEXT_LEVEL*]->(end:Question {question_number: $end_question}))
		RETURN [node in nodes(path) | node.question_number] as path,
			   reduce(totalWeight = 0, rel in relationships(path) | totalWeight + rel.weight) as total_weight,
			   length(path) as path_length
	`

	params := map[string]interface{}{
		"start_question": startQuestion,
		"end_question":   endQuestion,
	}

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			record := result.Record()
			pathNodes := record.Values[0].([]interface{})
			totalWeight := record.Values[1].(float64)
			pathLength := int(record.Values[2].(int64))

			path := make([]int, len(pathNodes))
			for i, node := range pathNodes {
				path[i] = int(node.(int64))
			}

			return &LearningPath{
				StartQuestion: startQuestion,
				EndQuestion:   endQuestion,
				Path:          path,
				TotalWeight:   totalWeight,
				PathLength:    pathLength,
			}, nil
		}

		return nil, nil // 没有找到路径
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return result.(*LearningPath), nil
}

// GetRecommendations 获取基于当前题目的推荐题目
func (s *QuestionGraphService) GetRecommendations(ctx context.Context, questionNumber int, limit int) ([]RecommendationResult, error) {
	query := `
		MATCH (current:Question {question_number: $question_number})
		MATCH (current)-[r:NEXT_LEVEL|SIMILAR]->(recommended:Question)
		WHERE recommended.status = 'published'
		RETURN recommended.question_number as question_number,
			   recommended.title as title,
			   recommended.difficulty as difficulty,
			   r.weight as score,
			   type(r) as relation_type,
			   CASE type(r)
				   WHEN 'NEXT_LEVEL' THEN '进阶题目'
				   WHEN 'SIMILAR' THEN '相似题目'
				   ELSE '相关题目'
			   END as reason
		ORDER BY r.weight DESC
		LIMIT $limit
	`

	params := map[string]interface{}{
		"question_number": questionNumber,
		"limit":          limit,
	}

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var recommendations []RecommendationResult
		for result.Next(ctx) {
			record := result.Record()
			recommendation := RecommendationResult{
				QuestionNumber: int(record.Values[0].(int64)),
				Title:          record.Values[1].(string),
				Difficulty:     record.Values[2].(string),
				Score:          record.Values[3].(float64),
				RelationType:   RelationshipType(record.Values[4].(string)),
				Reason:         record.Values[5].(string),
			}
			recommendations = append(recommendations, recommendation)
		}

		return recommendations, result.Err()
	})

	if err != nil {
		return nil, err
	}

	return result.([]RecommendationResult), nil
}