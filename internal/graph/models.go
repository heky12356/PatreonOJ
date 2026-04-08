package graph

import "time"

// QuestionNode represents a question node in graph storage.
type QuestionNode struct {
	QuestionNumber int       `json:"question_number"`
	QuestionId     string    `json:"question_id"`
	Title          string    `json:"title"`
	Difficulty     string    `json:"difficulty"`
	Tags           string    `json:"tags"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RelationshipType is the type for question relation edges.
type RelationshipType string

const (
	PREREQUISITE RelationshipType = "PREREQUISITE"
	NEXT_LEVEL   RelationshipType = "NEXT_LEVEL"
	SIMILAR      RelationshipType = "SIMILAR"

	TAG_SIMILAR  RelationshipType = "TAG_SIMILAR"
	TAG          RelationshipType = "TAG"
	TAG_CO_OCCUR RelationshipType = "TAG_CO_OCCUR"
)

// SkillRelationType is the type for skill-to-skill edges.
type SkillRelationType string

const (
	SKILL_SUBSUMES SkillRelationType = "SKILL_SUBSUMES"
	SKILL_CO_OCCUR SkillRelationType = "SKILL_CO_OCCUR"
)

// QuestionRelation represents a directed relation between two questions.
type QuestionRelation struct {
	FromQuestionNumber int              `json:"from_question_number"`
	ToQuestionNumber   int              `json:"to_question_number"`
	RelationType       RelationshipType `json:"relation_type"`
	Weight             float64          `json:"weight"`
	Description        string           `json:"description"`
	CreatedAt          time.Time        `json:"created_at"`
}

// LearningPath is a computed path between two questions.
type LearningPath struct {
	StartQuestion int     `json:"start_question"`
	EndQuestion   int     `json:"end_question"`
	Path          []int   `json:"path"`
	TotalWeight   float64 `json:"total_weight"`
	PathLength    int     `json:"path_length"`
}

// RecommendationResult is one recommendation item.
type RecommendationResult struct {
	QuestionNumber int              `json:"question_number"`
	QuestionId     string           `json:"question_id"`
	Title          string           `json:"title"`
	Difficulty     string           `json:"difficulty"`
	Score          float64          `json:"score"`
	Reason         string           `json:"reason"`
	RelationType   RelationshipType `json:"relation_type"`
	SkillKey       string           `json:"skill_key"`
}

// SkillNodeStatus is the learner status for one skill node.
type SkillNodeStatus struct {
	SkillKey      string   `json:"skill_key"`
	Mastery       float64  `json:"mastery"`
	Status        string   `json:"status"`
	Prerequisites []string `json:"prerequisites"`
}

