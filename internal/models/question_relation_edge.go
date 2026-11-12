package models

import "time"

type QuestionRelationEdgeType string

const (
	QuestionRelationEdgeTypePrerequisite QuestionRelationEdgeType = "PREREQUISITE"
	QuestionRelationEdgeTypeNextLevel    QuestionRelationEdgeType = "NEXT_LEVEL"
	QuestionRelationEdgeTypeSimilar      QuestionRelationEdgeType = "SIMILAR"
)

func (t QuestionRelationEdgeType) Valid() bool {
	switch t {
	case QuestionRelationEdgeTypePrerequisite,
		QuestionRelationEdgeTypeNextLevel,
		QuestionRelationEdgeTypeSimilar:
		return true
	default:
		return false
	}
}

type QuestionRelationEdge struct {
	ID uint `gorm:"primaryKey"`

	FromQuestionID string                   `gorm:"size:64;not null;index;uniqueIndex:idx_question_edge_unique;index:idx_edge_out,priority:1;index:idx_edge_in,priority:3" json:"from_question_id"`
	ToQuestionID   string                   `gorm:"size:64;not null;index;uniqueIndex:idx_question_edge_unique;index:idx_edge_out,priority:3;index:idx_edge_in,priority:1" json:"to_question_id"`
	EdgeType       QuestionRelationEdgeType `gorm:"size:32;not null;index;uniqueIndex:idx_question_edge_unique;index:idx_edge_out,priority:2;index:idx_edge_in,priority:2" json:"edge_type"`
	Source         string                   `gorm:"size:16;not null;default:legacy;index" json:"source"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (QuestionRelationEdge) TableName() string {
	return "question_relation_edges"
}
