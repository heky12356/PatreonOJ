package question

import (
	"log"
	"strings"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

// QuestionEdgeBuilder builds question relation edges after question creation.
type QuestionEdgeBuilder interface {
	Enqueue(questionNumber int)
}

// AsyncQuestionEdgeBuilder preserves the existing API. Rebuilding is synchronous
// so accepted edits cannot leave volatile background work behind on shutdown.
type AsyncQuestionEdgeBuilder struct {
	db *gorm.DB
}

// NewAsyncQuestionEdgeBuilder creates a channel-based async edge builder.
func NewAsyncQuestionEdgeBuilder(db *gorm.DB, queueSize int) QuestionEdgeBuilder {
	if db == nil {
		return nil
	}
	return &AsyncQuestionEdgeBuilder{db: db}
}

func (b *AsyncQuestionEdgeBuilder) Enqueue(questionNumber int) {
	if questionNumber <= 0 {
		return
	}
	if err := b.buildForQuestion(questionNumber); err != nil {
		log.Printf("question edge builder failed(question_number=%d): %v", questionNumber, err)
	}
}

func (b *AsyncQuestionEdgeBuilder) buildForQuestion(questionNumber int) error {
	return b.db.Transaction(func(tx *gorm.DB) error { return refreshQuestionEdges(tx, questionNumber) })
}

func refreshQuestionEdges(db *gorm.DB, questionNumber int) error {
	var current models.Question
	if err := db.Where("id = ?", questionNumber).First(&current).Error; err != nil {
		return err
	}
	var others []models.Question
	if err := db.Find(&others).Error; err != nil {
		return err
	}
	counts := map[string]int{}
	for _, other := range others {
		counts[other.QuestionId]++
	}
	if err := deleteQuestionEdges(db, current.QuestionId); err != nil {
		return err
	}
	// Duplicate business identifiers remain valid question records, but cannot
	// safely identify graph endpoints. Never guess which question they refer to.
	if counts[current.QuestionId] != 1 || strings.TrimSpace(current.QuestionId) == "" {
		return nil
	}
	currentTags := parseTagSet(current.Tags)
	if len(currentTags) == 0 {
		return nil
	}
	for _, other := range others {
		if other.Id == current.Id || counts[other.QuestionId] != 1 || strings.TrimSpace(other.QuestionId) == "" {
			continue
		}

		otherTags := parseTagSet(other.Tags)
		if len(otherTags) == 0 {
			continue
		}

		switch {
		case isStrictSubset(otherTags, currentTags):
			if err := createProgressionEdges(db, other.QuestionId, current.QuestionId); err != nil {
				return err
			}
		case isStrictSubset(currentTags, otherTags):
			if err := createProgressionEdges(db, current.QuestionId, other.QuestionId); err != nil {
				return err
			}
		case hasIntersection(currentTags, otherTags):
			if err := createSimilarEdges(db, current.QuestionId, other.QuestionId); err != nil {
				return err
			}
		}
	}

	return nil
}

func deleteQuestionEdges(db *gorm.DB, questionID string) error {
	return db.Where("from_question_id = ? OR to_question_id = ?", questionID, questionID).Delete(&models.QuestionRelationEdge{}).Error
}

func createProgressionEdges(db *gorm.DB, fromID, toID string) error {
	return db.Create(&[]models.QuestionRelationEdge{
		{FromQuestionID: fromID, ToQuestionID: toID, EdgeType: models.QuestionRelationEdgeTypePrerequisite},
		{FromQuestionID: fromID, ToQuestionID: toID, EdgeType: models.QuestionRelationEdgeTypeNextLevel},
	}).Error
}

func createSimilarEdges(db *gorm.DB, firstID, secondID string) error {
	return db.Create(&[]models.QuestionRelationEdge{
		{FromQuestionID: firstID, ToQuestionID: secondID, EdgeType: models.QuestionRelationEdgeTypeSimilar},
		{FromQuestionID: secondID, ToQuestionID: firstID, EdgeType: models.QuestionRelationEdgeTypeSimilar},
	}).Error
}

func parseTagSet(tags string) map[string]struct{} {
	set := make(map[string]struct{})
	if strings.TrimSpace(tags) == "" {
		return set
	}

	fields := strings.FieldsFunc(tags, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '|', '/', '\\':
			return true
		default:
			return false
		}
	})

	for _, f := range fields {
		tag := strings.ToLower(strings.TrimSpace(f))
		if tag == "" {
			continue
		}
		set[tag] = struct{}{}
	}
	return set
}

func isStrictSubset(a, b map[string]struct{}) bool {
	if len(a) == 0 || len(a) >= len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func hasIntersection(a, b map[string]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; ok {
			return true
		}
	}
	return false
}
