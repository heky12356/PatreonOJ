package graph

import "time"

// QuestionNode 题目节点结构
type QuestionNode struct {
	QuestionNumber int    `json:"question_number"`
	Title          string `json:"title"`
	Difficulty     string `json:"difficulty"`
	Tags           string `json:"tags"`
	Status         string `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RelationshipType 关系类型枚举
type RelationshipType string

const (
	// PREREQUISITE 前置关系：A是B的前置题目
	PREREQUISITE RelationshipType = "PREREQUISITE"
	// NEXT_LEVEL 进阶关系：从A可以进阶到B
	NEXT_LEVEL RelationshipType = "NEXT_LEVEL"
	// SIMILAR 相似关系：A和B是相似题目
	SIMILAR RelationshipType = "SIMILAR"
	// CATEGORY 分类关系：A和B属于同一类别
	CATEGORY RelationshipType = "CATEGORY"
)

// QuestionRelation 题目关系结构
type QuestionRelation struct {
	FromQuestionNumber int              `json:"from_question_number"`
	ToQuestionNumber   int              `json:"to_question_number"`
	RelationType       RelationshipType `json:"relation_type"`
	Weight             float64          `json:"weight"`     // 关系权重，用于推荐算法
	Description        string           `json:"description"` // 关系描述
	CreatedAt          time.Time        `json:"created_at"`
}

// LearningPath 学习路径结构
type LearningPath struct {
	StartQuestion int   `json:"start_question"`
	EndQuestion   int   `json:"end_question"`
	Path          []int `json:"path"`
	TotalWeight   float64 `json:"total_weight"`
	PathLength    int   `json:"path_length"`
}

// RecommendationResult 推荐结果结构
type RecommendationResult struct {
	QuestionNumber int     `json:"question_number"`
	Title          string  `json:"title"`
	Difficulty     string  `json:"difficulty"`
	Score          float64 `json:"score"`          // 推荐分数
	Reason         string  `json:"reason"`         // 推荐理由
	RelationType   RelationshipType `json:"relation_type"`
}