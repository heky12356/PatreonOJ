package graph

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"dachuang/internal/models"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"gorm.io/gorm"
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

// ListQuestions 查询所有题目节点
func (s *QuestionGraphService) ListQuestions(ctx context.Context) ([]QuestionNode, error) {
	query := `
		MATCH (q:Question)
		RETURN q.question_number as question_number,
			   coalesce(q.question_id, '') as question_id,
			   q.title as title,
			   q.difficulty as difficulty,
			   q.tags as tags,
			   q.status as status,
			   q.created_at as created_at,
			   q.updated_at as updated_at
		ORDER BY q.question_number
	`

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var questions []QuestionNode
		for result.Next(ctx) {
			record := result.Record()

			var createdAt, updatedAt time.Time
			if record.Values[6] != nil {
				if t, ok := record.Values[6].(time.Time); ok {
					createdAt = t
				}
			}
			if record.Values[7] != nil {
				if t, ok := record.Values[7].(time.Time); ok {
					updatedAt = t
				}
			}

			question := QuestionNode{
				QuestionNumber: int(record.Values[0].(int64)),
				QuestionId:     record.Values[1].(string),
				Title:          record.Values[2].(string),
				Difficulty:     record.Values[3].(string),
				Tags:           record.Values[4].(string),
				Status:         record.Values[5].(string),
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

// QuestionSkillRelation 题目-技能关系结构
type QuestionSkillRelation struct {
	QuestionNumber int     `json:"question_number"`
	SkillKey       string  `json:"skill_key"`
	Weight         float64 `json:"weight"`
}

// SkillRelation 技能-技能关系结构
type SkillRelation struct {
	FromKey      string            `json:"from_key"`
	ToKey        string            `json:"to_key"`
	RelationType SkillRelationType `json:"relation_type"`
	Weight       float64           `json:"weight"`
}

// ListSkills 查询所有技能节点
func (s *QuestionGraphService) ListSkills(ctx context.Context) ([]SkillNode, error) {
	query := `
		MATCH (s:Skill)
		RETURN s.key as key,
			s.name as name,
			s.created_at as created_at,
			s.updated_at as updated_at
		ORDER BY s.name
	`

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var skills []SkillNode
		for result.Next(ctx) {
			record := result.Record()

			var createdAt, updatedAt time.Time
			if record.Values[2] != nil {
				if t, ok := record.Values[2].(time.Time); ok {
					createdAt = t
				}
			}
			if record.Values[3] != nil {
				if t, ok := record.Values[3].(time.Time); ok {
					updatedAt = t
				}
			}

			skill := SkillNode{
				Key:       record.Values[0].(string),
				Name:      record.Values[1].(string),
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			}
			skills = append(skills, skill)
		}

		return skills, result.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]SkillNode), nil
}

// ListQuestionSkillRelations 查询所有题目-技能关系
func (s *QuestionGraphService) ListQuestionSkillRelations(ctx context.Context) ([]QuestionSkillRelation, error) {
	query := `
		MATCH (q:Question)-[r:HAS_SKILL]->(s:Skill)
		RETURN q.question_number as question_number,
			s.key as skill_key,
			coalesce(r.weight, 1) as weight
		ORDER BY question_number, skill_key
	`

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var relations []QuestionSkillRelation
		for result.Next(ctx) {
			record := result.Record()

			var weight float64
			switch v := record.Values[2].(type) {
			case float64:
				weight = v
			case int64:
				weight = float64(v)
			case int:
				weight = float64(v)
			}

			relations = append(relations, QuestionSkillRelation{
				QuestionNumber: int(record.Values[0].(int64)),
				SkillKey:       record.Values[1].(string),
				Weight:         weight,
			})
		}

		return relations, result.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]QuestionSkillRelation), nil
}

// ListSkillRelations 查询所有技能-技能关系
func (s *QuestionGraphService) ListSkillRelations(ctx context.Context) ([]SkillRelation, error) {
	query := `
		MATCH (a:Skill)-[r:SKILL_CO_OCCUR|SKILL_SUBSUMES]->(b:Skill)
		RETURN a.key as from_key,
			b.key as to_key,
			type(r) as relation_type,
			coalesce(r.weight, 0) as weight
		ORDER BY relation_type, from_key, to_key
	`

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var relations []SkillRelation
		for result.Next(ctx) {
			record := result.Record()

			var weight float64
			switch v := record.Values[3].(type) {
			case float64:
				weight = v
			case int64:
				weight = float64(v)
			case int:
				weight = float64(v)
			}

			relations = append(relations, SkillRelation{
				FromKey:      record.Values[0].(string),
				ToKey:        record.Values[1].(string),
				RelationType: SkillRelationType(record.Values[2].(string)),
				Weight:       weight,
			})
		}

		return relations, result.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]SkillRelation), nil
}

// ListRelations 查询所有题目关系
func (s *QuestionGraphService) ListRelations(ctx context.Context) ([]QuestionRelation, error) {
	query := `
		MATCH (from:Question)-[r]->(to:Question)
		WHERE type(r) IN $types
		RETURN from.question_number as from_question_number,
			   to.question_number as to_question_number,
			   type(r) as relation_type,
			   r.weight as weight,
			   r.description as description,
			   r.created_at as created_at
		ORDER BY from_question_number, to_question_number, relation_type
	`

	types := []string{string(PREREQUISITE), string(NEXT_LEVEL), string(SIMILAR), string(CATEGORY), string(TAG_SIMILAR)}
	params := map[string]interface{}{"types": types}

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var relations []QuestionRelation
		for result.Next(ctx) {
			record := result.Record()

			var weight float64
			switch v := record.Values[3].(type) {
			case float64:
				weight = v
			case int64:
				weight = float64(v)
			case int:
				weight = float64(v)
			}

			var description string
			if record.Values[4] != nil {
				if s, ok := record.Values[4].(string); ok {
					description = s
				}
			}

			var createdAt time.Time
			if record.Values[5] != nil {
				if t, ok := record.Values[5].(time.Time); ok {
					createdAt = t
				}
			}

			relation := QuestionRelation{
				FromQuestionNumber: int(record.Values[0].(int64)),
				ToQuestionNumber:   int(record.Values[1].(int64)),
				RelationType:       RelationshipType(record.Values[2].(string)),
				Weight:             weight,
				Description:        description,
				CreatedAt:          createdAt,
			}
			relations = append(relations, relation)
		}

		return relations, result.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]QuestionRelation), nil
}

// CreateOrUpdateQuestion 创建或更新题目节点
func (s *QuestionGraphService) CreateOrUpdateQuestion(ctx context.Context, question QuestionNode) error {
	query := `
		MERGE (q:Question {question_number: $question_number})
		ON CREATE SET 
			q.question_id = $question_id,
			q.title = $title,
			q.difficulty = $difficulty,
			q.tags = $tags,
			q.status = $status,
			q.created_at = $created_at,
			q.updated_at = $updated_at
		ON MATCH SET 
			q.question_id = $question_id,
			q.title = $title,
			q.difficulty = $difficulty,
			q.tags = $tags,
			q.status = $status,
			q.updated_at = $updated_at
	`

	params := map[string]interface{}{
		"question_number": question.QuestionNumber,
		"question_id":     question.QuestionId,
		"title":           question.Title,
		"difficulty":      question.Difficulty,
		"tags":            question.Tags,
		"status":          question.Status,
		"created_at":      question.CreatedAt,
		"updated_at":      question.UpdatedAt,
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
		"weight":               relation.Weight,
		"description":          relation.Description,
		"created_at":           relation.CreatedAt,
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
		RETURN pre.question_number as question_number,
			   coalesce(pre.question_id, '') as question_id,
			   pre.title as title,
			   pre.difficulty as difficulty, 
			   pre.tags as tags,
			   pre.status as status,
			   pre.created_at as created_at,
			   pre.updated_at as updated_at
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
			if record.Values[6] != nil {
				if t, ok := record.Values[6].(time.Time); ok {
					createdAt = t
				}
			}
			if record.Values[7] != nil {
				if t, ok := record.Values[7].(time.Time); ok {
					updatedAt = t
				}
			}

			question := QuestionNode{
				QuestionNumber: int(record.Values[0].(int64)),
				QuestionId:     record.Values[1].(string),
				Title:          record.Values[2].(string),
				Difficulty:     record.Values[3].(string),
				Tags:           record.Values[4].(string),
				Status:         record.Values[5].(string),
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
		RETURN next.question_number as question_number,
			   coalesce(next.question_id, '') as question_id,
			   next.title as title,
			   next.difficulty as difficulty,
			   next.tags as tags,
			   next.status as status,
			   next.created_at as created_at,
			   next.updated_at as updated_at
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
			if record.Values[6] != nil {
				if t, ok := record.Values[6].(time.Time); ok {
					createdAt = t
				}
			}
			if record.Values[7] != nil {
				if t, ok := record.Values[7].(time.Time); ok {
					updatedAt = t
				}
			}

			question := QuestionNode{
				QuestionNumber: int(record.Values[0].(int64)),
				QuestionId:     record.Values[1].(string),
				Title:          record.Values[2].(string),
				Difficulty:     record.Values[3].(string),
				Tags:           record.Values[4].(string),
				Status:         record.Values[5].(string),
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
		CALL {
			WITH $question_number AS qn
			MATCH (current:Question {question_number: qn})
			MATCH (current)-[r:NEXT_LEVEL|SIMILAR]->(recommended:Question)
			WHERE recommended.status = 'published'
			RETURN recommended, toFloat(coalesce(r.weight, 0)) AS score, type(r) AS relation_type,
				CASE type(r)
					WHEN 'NEXT_LEVEL' THEN '进阶题目'
					WHEN 'SIMILAR' THEN '相似题目'
					ELSE '相关题目'
				END AS reason
			UNION
			WITH $question_number AS qn
			MATCH (current:Question {question_number: qn})-[:HAS_SKILL]->(s:Skill)<-[:HAS_SKILL]-(recommended:Question)
			WHERE recommended.status = 'published' AND recommended.question_number <> qn
			WITH recommended, count(DISTINCT s) AS shared
			RETURN recommended, toFloat(shared) AS score, 'TAG' AS relation_type,
				'同标签: ' + toString(shared) + ' 个' AS reason
			UNION
			WITH $question_number AS qn
			MATCH (current:Question {question_number: qn})-[:HAS_SKILL]->(:Skill)-[r:SKILL_CO_OCCUR]-(:Skill)<-[:HAS_SKILL]-(recommended:Question)
			WHERE recommended.status = 'published' AND recommended.question_number <> qn
			WITH recommended, sum(toFloat(coalesce(r.weight, 1))) AS w
			RETURN recommended, w AS score, 'TAG_CO_OCCUR' AS relation_type,
				'共现标签' AS reason
		}
		WITH recommended, score, relation_type, reason
		ORDER BY score DESC
		WITH recommended, collect({score: score, relation_type: relation_type, reason: reason})[0] AS best
		RETURN recommended.question_number as question_number,
			coalesce(recommended.question_id, '') as question_id,
			recommended.title as title,
			recommended.difficulty as difficulty,
			best.score as score,
			best.relation_type as relation_type,
			best.reason as reason
		ORDER BY score DESC
		LIMIT $limit
	`

	params := map[string]interface{}{
		"question_number": questionNumber,
		"limit":           limit,
	}

	result, err := s.client.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var recommendations []RecommendationResult
		for result.Next(ctx) {
			record := result.Record()

			var score float64
			switch v := record.Values[4].(type) {
			case float64:
				score = v
			case int64:
				score = float64(v)
			case int:
				score = float64(v)
			}

			recommendation := RecommendationResult{
				QuestionNumber: int(record.Values[0].(int64)),
				QuestionId:     record.Values[1].(string),
				Title:          record.Values[2].(string),
				Difficulty:     record.Values[3].(string),
				Score:          score,
				RelationType:   RelationshipType(record.Values[5].(string)),
				Reason:         record.Values[6].(string),
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

// InitGraph 初始化图数据库，确保所有题目节点和关系都存在
func (s *QuestionGraphService) InitGraph(ctx context.Context, db *gorm.DB) error {
	var questions []models.Question
	if err := db.Order("question_number ASC").Find(&questions).Error; err != nil {
		return fmt.Errorf("读取题目列表失败: %w", err)
	}

	graphQuestions, err := s.ListQuestions(ctx)
	if err != nil {
		return fmt.Errorf("读取图数据库题目节点失败: %w", err)
	}

	dbQuestionMap := make(map[int]models.Question, len(questions))
	for _, q := range questions {
		dbQuestionMap[q.QuestionNumber] = q
	}

	graphQuestionMap := make(map[int]QuestionNode, len(graphQuestions))
	for _, q := range graphQuestions {
		graphQuestionMap[q.QuestionNumber] = q
	}

	now := time.Now().UTC()
	for _, q := range questions {
		desired := QuestionNode{
			QuestionNumber: q.QuestionNumber,
			QuestionId:     q.QuestionId,
			Title:          q.Title,
			Difficulty:     q.Difficulty,
			Tags:           q.Tags,
			Status:         q.Status,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		existing, ok := graphQuestionMap[q.QuestionNumber]
		if ok && sameQuestionNode(existing, desired) {
			continue
		}

		if err := s.CreateOrUpdateQuestion(ctx, desired); err != nil {
			return fmt.Errorf("同步题目节点失败(question_number=%d): %w", q.QuestionNumber, err)
		}
	}

	for number := range graphQuestionMap {
		if _, ok := dbQuestionMap[number]; ok {
			continue
		}
		if err := s.DeleteQuestion(ctx, number); err != nil {
			return fmt.Errorf("删除多余题目节点失败(question_number=%d): %w", number, err)
		}
	}

	var dbRelations []models.Relation
	if err := db.Find(&dbRelations).Error; err != nil {
		return fmt.Errorf("读取关系表失败: %w", err)
	}

	validTypes := map[RelationshipType]bool{
		PREREQUISITE: true,
		NEXT_LEVEL:   true,
		SIMILAR:      true,
		CATEGORY:     true,
	}

	dbRelationMap := make(map[string]QuestionRelation, len(dbRelations))
	for _, r := range dbRelations {
		relType := RelationshipType(r.Relation)
		if !validTypes[relType] {
			continue
		}
		if _, ok := dbQuestionMap[r.SourceID]; !ok {
			continue
		}
		if _, ok := dbQuestionMap[r.TargetID]; !ok {
			continue
		}

		rel := QuestionRelation{
			FromQuestionNumber: r.SourceID,
			ToQuestionNumber:   r.TargetID,
			RelationType:       relType,
			Weight:             1,
			Description:        "",
			CreatedAt:          now,
		}
		dbRelationMap[relationKey(rel.FromQuestionNumber, rel.ToQuestionNumber, rel.RelationType)] = rel
	}

	graphRelations, err := s.ListRelations(ctx)
	if err != nil {
		return fmt.Errorf("读取图数据库关系失败: %w", err)
	}
	graphRelationMap := make(map[string]QuestionRelation, len(graphRelations))
	for _, r := range graphRelations {
		graphRelationMap[relationKey(r.FromQuestionNumber, r.ToQuestionNumber, r.RelationType)] = r
	}

	for key, rel := range dbRelationMap {
		if _, ok := graphRelationMap[key]; ok {
			continue
		}
		if err := s.CreateRelation(ctx, rel); err != nil {
			return fmt.Errorf("创建缺失关系失败(%s): %w", key, err)
		}
	}

	for key, rel := range graphRelationMap {
		if _, ok := dbRelationMap[key]; ok {
			continue
		}
		if !validTypes[rel.RelationType] {
			continue
		}
		if err := s.DeleteRelation(ctx, rel.FromQuestionNumber, rel.ToQuestionNumber, rel.RelationType); err != nil {
			return fmt.Errorf("删除多余关系失败(%s): %w", key, err)
		}
	}

	// 题目节点/题目关系同步完成后，进一步同步“技能/标签( Skill )节点”与自动关系。
	// 这样图能覆盖“知识点维度”，便于做同标签推荐、薄弱知识点补题、以及更稳定的学习路径。
	if err := s.SyncSkills(ctx, questions, now); err != nil {
		return fmt.Errorf("同步技能节点失败: %w", err)
	}

	if err := s.SyncTagSimilarEdges(ctx, now); err != nil {
		return fmt.Errorf("同步同标签题目关系失败: %w", err)
	}

	return nil
}

// sameQuestionNode 检查两个题目节点是否相同
func sameQuestionNode(a, b QuestionNode) bool {
	return a.QuestionNumber == b.QuestionNumber &&
		a.QuestionId == b.QuestionId &&
		a.Title == b.Title &&
		a.Difficulty == b.Difficulty &&
		a.Tags == b.Tags &&
		a.Status == b.Status
}

// relationKey 生成关系键，用于唯一标识关系
func relationKey(from, to int, relationType RelationshipType) string {
	return fmt.Sprintf("%d->%d:%s", from, to, relationType)
}

// SkillNode 技能/标签节点
type SkillNode struct {
	Key       string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SkillRelationType 技能/标签关系类型
type SkillRelationType string

const (
	SkillRelCoOccur  SkillRelationType = "SKILL_CO_OCCUR" // 同题共现（权重=共现次数）
	SkillRelSubsumes SkillRelationType = "SKILL_SUBSUMES" // 基于名称包含的泛化/细化关系（权重固定）
	QuestionHasSkill SkillRelationType = "HAS_SKILL"      // 题目-技能关系（权重=1）
)

// skillRel 技能/标签关系
type skillRel struct {
	FromKey string
	ToKey   string
	Type    SkillRelationType
	Weight  float64
}

// SyncQuestionSkills 同步题目-技能关系
func (s *QuestionGraphService) SyncQuestionSkills(ctx context.Context, questionNumber int, tags string, now time.Time) error {
	if questionNumber == 0 {
		return nil
	}

	constraintQuery := `CREATE CONSTRAINT skill_key IF NOT EXISTS FOR (s:Skill) REQUIRE s.key IS UNIQUE`
	if _, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, constraintQuery, nil)
		return nil, err
	}); err != nil {
		return err
	}

	keys := uniqueSortedSkillKeysFromTags(tags)

	if len(keys) == 0 {
		query := `
			MATCH (q:Question {question_number: $question_number})-[r:HAS_SKILL {auto: true}]->(:Skill)
			DELETE r
		`
		params := map[string]interface{}{"question_number": questionNumber}
		_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			_, err := tx.Run(ctx, query, params)
			return nil, err
		})
		return err
	}

	skills := make([]map[string]interface{}, 0, len(keys))
	for _, k := range keys {
		name := displayNameFromKey(k)
		skills = append(skills, map[string]interface{}{
			"key":        k,
			"name":       name,
			"created_at": now,
			"updated_at": now,
		})
	}

	upsertSkillsQuery := `
		UNWIND $skills AS sk
		MERGE (s:Skill {key: sk.key})
		ON CREATE SET s.name = sk.name, s.created_at = sk.created_at, s.updated_at = sk.updated_at
		ON MATCH SET  s.name = sk.name, s.updated_at = sk.updated_at
	`
	if _, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, upsertSkillsQuery, map[string]interface{}{"skills": skills})
		return nil, err
	}); err != nil {
		return err
	}

	upsertEdgesQuery := `
		MERGE (q:Question {question_number: $question_number})
		WITH q
		OPTIONAL MATCH (q)-[r:HAS_SKILL {auto: true}]->(:Skill)
		DELETE r
		WITH q
		UNWIND $skills AS sk
		MERGE (s:Skill {key: sk.key})
		ON CREATE SET s.name = sk.name, s.created_at = $now, s.updated_at = $now
		ON MATCH SET  s.name = sk.name, s.updated_at = $now
		MERGE (q)-[rel:HAS_SKILL]->(s)
		SET rel.weight = 1.0,
			rel.auto = true,
			rel.created_at = coalesce(rel.created_at, $now),
			rel.updated_at = $now
	`
	params := map[string]interface{}{"question_number": questionNumber, "skills": skills, "now": now}
	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, upsertEdgesQuery, params)
		return nil, err
	})
	return err
}

// BuildTagSimilarEdgesForQuestion 为题目建立基于标签的相似关系
func (s *QuestionGraphService) BuildTagSimilarEdgesForQuestion(ctx context.Context, questionNumber int, now time.Time) error {
	if questionNumber == 0 {
		return nil
	}

	cleanupQuery := `
		MATCH (q:Question {question_number: $question_number})-[r:TAG_SIMILAR {auto: true}]->(:Question)
		DELETE r
	`
	cleanupInQuery := `
		MATCH (:Question)-[r:TAG_SIMILAR {auto: true}]->(q:Question {question_number: $question_number})
		DELETE r
	`
	params := map[string]interface{}{"question_number": questionNumber}
	if _, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		if _, err := tx.Run(ctx, cleanupQuery, params); err != nil {
			return nil, err
		}
		_, err := tx.Run(ctx, cleanupInQuery, params)
		return nil, err
	}); err != nil {
		return err
	}

	query := `
		MATCH (q:Question {question_number: $question_number})-[:HAS_SKILL]->(s:Skill)<-[:HAS_SKILL]-(o:Question)
		WHERE o.question_number <> $question_number AND o.status = 'published'
		WITH q, o, count(DISTINCT s) AS shared
		MERGE (q)-[r:TAG_SIMILAR]->(o)
		SET r.weight = toFloat(shared),
			r.auto = true,
			r.description = 'shared_tags:' + toString(shared),
			r.created_at = coalesce(r.created_at, $now),
			r.updated_at = $now
		MERGE (o)-[r2:TAG_SIMILAR]->(q)
		SET r2.weight = toFloat(shared),
			r2.auto = true,
			r2.description = 'shared_tags:' + toString(shared),
			r2.created_at = coalesce(r2.created_at, $now),
			r2.updated_at = $now
	`
	params2 := map[string]interface{}{"question_number": questionNumber, "now": now}
	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params2)
		return nil, err
	})
	return err
}

// SyncTagSimilarEdges 同步所有题目之间的基于标签的相似关系
func (s *QuestionGraphService) SyncTagSimilarEdges(ctx context.Context, now time.Time) error {
	cleanupQuery := `
		MATCH (:Question)-[r:TAG_SIMILAR {auto: true}]->(:Question)
		DELETE r
	`
	if _, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, cleanupQuery, nil)
		return nil, err
	}); err != nil {
		return err
	}

	query := `
		MATCH (q:Question)-[:HAS_SKILL]->(s:Skill)<-[:HAS_SKILL]-(o:Question)
		WHERE q.status = 'published' AND o.status = 'published' AND q.question_number < o.question_number
		WITH q, o, count(DISTINCT s) AS shared
		WHERE shared > 0
		MERGE (q)-[r:TAG_SIMILAR]->(o)
		SET r.weight = toFloat(shared),
			r.auto = true,
			r.description = 'shared_tags:' + toString(shared),
			r.created_at = coalesce(r.created_at, $now),
			r.updated_at = $now
		MERGE (o)-[r2:TAG_SIMILAR]->(q)
		SET r2.weight = toFloat(shared),
			r2.auto = true,
			r2.description = 'shared_tags:' + toString(shared),
			r2.created_at = coalesce(r2.created_at, $now),
			r2.updated_at = $now
	`
	params := map[string]interface{}{"now": now}
	_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// SyncSkills 将数据库中题目的 Tags 自动同步为图数据库中的 Skill 节点，并建立：
// 1) (Question)-[:HAS_SKILL]->(Skill) 边（严格与题目 tags 保持一致）
// 2) Skill 间自动关系：
//   - SKILL_CO_OCCUR：同题共现（权重=共现次数）
//   - SKILL_SUBSUMES：基于名称包含的泛化/细化关系（权重固定）
func (s *QuestionGraphService) SyncSkills(ctx context.Context, questions []models.Question, now time.Time) error {
	// 1) 解析题目标签，构建去重后的 Skill 列表，以及每题对应的 skill keys
	skillsByKey := map[string]SkillNode{}
	questionSkills := make([]map[string]interface{}, 0, len(questions))
	for _, q := range questions {
		if q.QuestionNumber == 0 {
			continue
		}
		keys := uniqueSortedSkillKeysFromTags(q.Tags)
		row := map[string]interface{}{
			"question_number": q.QuestionNumber,
			"skills":          make([]map[string]interface{}, 0, len(keys)),
		}
		for _, k := range keys {
			name := displayNameFromKey(k)
			skillsByKey[k] = SkillNode{Key: k, Name: name, CreatedAt: now, UpdatedAt: now}
			row["skills"] = append(row["skills"].([]map[string]interface{}), map[string]interface{}{
				"key":    k,
				"name":   name,
				"weight": 1.0,
			})
		}
		questionSkills = append(questionSkills, row)
	}

	// 2) 创建唯一约束（如果Neo4j已存在则跳过）
	constraintQuery := `CREATE CONSTRAINT skill_key IF NOT EXISTS FOR (s:Skill) REQUIRE s.key IS UNIQUE`
	if _, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, constraintQuery, nil)
		return nil, err
	}); err != nil {
		return err
	}

	// 3) 批量导入 Skill 节点（去重后）
	skillList := make([]map[string]interface{}, 0, len(skillsByKey))
	for _, sk := range skillsByKey {
		skillList = append(skillList, map[string]interface{}{
			"key":        sk.Key,
			"name":       sk.Name,
			"created_at": sk.CreatedAt,
			"updated_at": sk.UpdatedAt,
		})
	}
	batchUpsertSkills := func(batch []map[string]interface{}) error {
		query := `
			UNWIND $skills AS sk
			MERGE (s:Skill {key: sk.key})
			ON CREATE SET s.name = sk.name, s.created_at = sk.created_at, s.updated_at = sk.updated_at
			ON MATCH SET  s.name = sk.name, s.updated_at = sk.updated_at
		`
		params := map[string]interface{}{"skills": batch}
		_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			_, err := tx.Run(ctx, query, params)
			return nil, err
		})
		return err
	}
	for i := 0; i < len(skillList); i += 500 {
		end := i + 500
		if end > len(skillList) {
			end = len(skillList)
		}
		if err := batchUpsertSkills(skillList[i:end]); err != nil {
			return err
		}
	}

	// 4) 为每道题同步 (Question)-[:HAS_SKILL]->(Skill) 边：
	//    先删后建，保证与当前 tags 完全一致（边带 auto=true 便于识别）。
	batchUpsertQuestionSkillEdges := func(batch []map[string]interface{}) error {
		query := `
			UNWIND $rows AS row
			MERGE (q:Question {question_number: row.question_number})
			WITH q, row
			OPTIONAL MATCH (q)-[r:HAS_SKILL {auto: true}]->(:Skill)
			DELETE r
			WITH q, row
			UNWIND row.skills AS sk
			MERGE (s:Skill {key: sk.key})
			ON CREATE SET s.name = sk.name, s.created_at = $now, s.updated_at = $now
			ON MATCH SET  s.name = sk.name, s.updated_at = $now
			MERGE (q)-[rel:HAS_SKILL]->(s)
			SET rel.weight = sk.weight,
				rel.auto = true,
				rel.created_at = coalesce(rel.created_at, $now),
				rel.updated_at = $now
		`
		params := map[string]interface{}{"rows": batch, "now": now}
		_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			_, err := tx.Run(ctx, query, params)
			return nil, err
		})
		return err
	}
	for i := 0; i < len(questionSkills); i += 200 {
		end := i + 200
		if end > len(questionSkills) {
			end = len(questionSkills)
		}
		if err := batchUpsertQuestionSkillEdges(questionSkills[i:end]); err != nil {
			return err
		}
	}

	// 5) 自动建立 Skill 间关系
	rels := buildAutoSkillRelations(questions)

	// 仅维护自动生成的关系：先清理再重建，避免历史残留。
	cleanupQuery := `
		MATCH (:Skill)-[r:SKILL_CO_OCCUR|SKILL_SUBSUMES {auto: true}]->(:Skill)
		DELETE r
	`
	if _, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, cleanupQuery, nil)
		return nil, err
	}); err != nil {
		return err
	}

	batchUpsertSkillRels := func(batch []map[string]interface{}, relType SkillRelationType) error {
		query := fmt.Sprintf(`
			UNWIND $rels AS rel
			MERGE (a:Skill {key: rel.from})
			MERGE (b:Skill {key: rel.to})
			MERGE (a)-[r:%s]->(b)
			SET r.weight = rel.weight,
				r.auto = true,
				r.created_at = coalesce(r.created_at, $now),
				r.updated_at = $now
		`, relType)
		params := map[string]interface{}{"rels": batch, "now": now}
		_, err := s.client.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			_, err := tx.Run(ctx, query, params)
			return nil, err
		})
		return err
	}

	coOccurBatch := make([]map[string]interface{}, 0)
	subsumesBatch := make([]map[string]interface{}, 0)
	for _, r := range rels {
		payload := map[string]interface{}{"from": r.FromKey, "to": r.ToKey, "weight": r.Weight}
		switch r.Type {
		case SkillRelCoOccur:
			coOccurBatch = append(coOccurBatch, payload)
		case SkillRelSubsumes:
			subsumesBatch = append(subsumesBatch, payload)
		}
	}

	for i := 0; i < len(coOccurBatch); i += 500 {
		end := i + 500
		if end > len(coOccurBatch) {
			end = len(coOccurBatch)
		}
		if err := batchUpsertSkillRels(coOccurBatch[i:end], SkillRelCoOccur); err != nil {
			return err
		}
	}
	for i := 0; i < len(subsumesBatch); i += 500 {
		end := i + 500
		if end > len(subsumesBatch) {
			end = len(subsumesBatch)
		}
		if err := batchUpsertSkillRels(subsumesBatch[i:end], SkillRelSubsumes); err != nil {
			return err
		}
	}

	return nil
}

// uniqueSortedSkillKeysFromTags 从 tags 字符串中提取唯一、排序后的技能键（小写、去空格）
func uniqueSortedSkillKeysFromTags(tags string) []string {
	tags = strings.ReplaceAll(tags, "，", ",")
	parts := strings.Split(tags, ",")
	set := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		k := normalizeSkillKey(p)
		if k == "" {
			continue
		}
		set[k] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// normalizeSkillKey 标准化技能键（小写、去空格、替换全角空格）
func normalizeSkillKey(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "　", " ")
	name = strings.Join(strings.Fields(name), " ")
	return strings.ToLower(name)
}

// displayNameFromKey 从技能键中提取显示名称（去空格）
func displayNameFromKey(key string) string {
	return strings.TrimSpace(key)
}

// buildAutoSkillRelations 根据题目的 tags 自动生成 Skill 间关系：
// 1) 同题共现：A与B在同一题出现则增加共现次数（权重=次数）。
// 2) 名称包含：若短标签是长标签的子串（且短标签至少2字符），建立泛化->细化关系。
func buildAutoSkillRelations(questions []models.Question) []skillRel {
	coOccur := map[string]int{}
	skillNames := map[string]string{}

	for _, q := range questions {
		keys := uniqueSortedSkillKeysFromTags(q.Tags)
		for _, k := range keys {
			skillNames[k] = k
		}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				a := keys[i]
				b := keys[j]
				coOccur[a+"|"+b]++
			}
		}
	}

	rels := make([]skillRel, 0, len(coOccur))
	for k, cnt := range coOccur {
		parts := strings.Split(k, "|")
		if len(parts) != 2 {
			continue
		}
		rels = append(rels, skillRel{FromKey: parts[0], ToKey: parts[1], Type: SkillRelCoOccur, Weight: float64(cnt)})
	}

	// SUBSUMES 关系（泛化->细化）
	all := make([]string, 0, len(skillNames))
	for k := range skillNames {
		all = append(all, k)
	}
	sort.Slice(all, func(i, j int) bool { return len([]rune(all[i])) < len([]rune(all[j])) })

	added := map[string]struct{}{}
	for i := 0; i < len(all); i++ {
		general := all[i]
		if len([]rune(general)) < 2 {
			continue
		}
		for j := i + 1; j < len(all); j++ {
			specific := all[j]
			if general == specific {
				continue
			}
			if strings.Contains(specific, general) {
				key := general + "->" + specific
				if _, ok := added[key]; ok {
					continue
				}
				added[key] = struct{}{}
				rels = append(rels, skillRel{FromKey: general, ToKey: specific, Type: SkillRelSubsumes, Weight: 0.6})
			}
		}
	}

	return rels
}
