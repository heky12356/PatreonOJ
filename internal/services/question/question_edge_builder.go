package question

import (
	"errors"
	"log"
	"strings"
	"time"

	"patreonoj/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QuestionEdgeBuilder builds question relation edges after question creation.
type QuestionEdgeBuilder interface {
	Enqueue(questionNumber int)
}

// AsyncQuestionEdgeBuilder handles edge building asynchronously.
type AsyncQuestionEdgeBuilder struct {
	db    *gorm.DB
	queue chan int
}

// NewAsyncQuestionEdgeBuilder creates a channel-based async edge builder.
func NewAsyncQuestionEdgeBuilder(db *gorm.DB, queueSize int) QuestionEdgeBuilder {
	if db == nil {
		return nil
	}
	if queueSize <= 0 {
		queueSize = 128
	}

	builder := &AsyncQuestionEdgeBuilder{
		db:    db,
		queue: make(chan int, queueSize),
	}
	go builder.run()
	return builder
}

func (b *AsyncQuestionEdgeBuilder) Enqueue(questionNumber int) {
	if questionNumber <= 0 {
		return
	}
	select {
	case b.queue <- questionNumber:
	default:
		go func() { b.queue <- questionNumber }()
	}
}

func (b *AsyncQuestionEdgeBuilder) run() {
	for questionNumber := range b.queue {
		if err := b.buildForQuestion(questionNumber); err != nil {
			log.Printf("question edge builder failed(question_number=%d): %v", questionNumber, err)
		}
	}
}

func (b *AsyncQuestionEdgeBuilder) buildForQuestion(questionNumber int) error {
	var current models.Question
	if err := b.db.Where("id = ?", questionNumber).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if strings.TrimSpace(current.QuestionId) == "" {
		return nil
	}

	currentTags := parseTagSet(current.Tags)
	if len(currentTags) == 0 {
		return nil
	}

	var others []models.Question
	if err := b.db.Where("id <> ?", current.Id).Find(&others).Error; err != nil {
		return err
	}

	for _, other := range others {
		if strings.TrimSpace(other.QuestionId) == "" {
			continue
		}

		otherTags := parseTagSet(other.Tags)
		if len(otherTags) == 0 {
			continue
		}

		switch {
		case isStrictSubset(otherTags, currentTags):
			b.createProgressionEdges(other.QuestionId, current.QuestionId)
		case isStrictSubset(currentTags, otherTags):
			b.createProgressionEdges(current.QuestionId, other.QuestionId)
		case hasIntersection(currentTags, otherTags):
			b.createSimilarEdges(current.QuestionId, other.QuestionId)
		}
	}

	return nil
}

func (b *AsyncQuestionEdgeBuilder) createProgressionEdges(fromQuestionID, toQuestionID string) {
	if err := b.upsertEdge(fromQuestionID, toQuestionID, models.QuestionRelationEdgeTypePrerequisite); err != nil {
		log.Printf("create PREREQUISITE relation failed(%s->%s): %v", fromQuestionID, toQuestionID, err)
	}
	if err := b.upsertEdge(fromQuestionID, toQuestionID, models.QuestionRelationEdgeTypeNextLevel); err != nil {
		log.Printf("create NEXT_LEVEL relation failed(%s->%s): %v", fromQuestionID, toQuestionID, err)
	}
}

func (b *AsyncQuestionEdgeBuilder) createSimilarEdges(questionAID, questionBID string) {
	if err := b.upsertEdge(questionAID, questionBID, models.QuestionRelationEdgeTypeSimilar); err != nil {
		log.Printf("create SIMILAR relation failed(%s->%s): %v", questionAID, questionBID, err)
	}
	if err := b.upsertEdge(questionBID, questionAID, models.QuestionRelationEdgeTypeSimilar); err != nil {
		log.Printf("create SIMILAR relation failed(%s->%s): %v", questionBID, questionAID, err)
	}
}

func (b *AsyncQuestionEdgeBuilder) upsertEdge(fromQuestionID, toQuestionID string, edgeType models.QuestionRelationEdgeType) error {
	now := time.Now().UTC()
	edge := models.QuestionRelationEdge{
		FromQuestionID: fromQuestionID,
		ToQuestionID:   toQuestionID,
		EdgeType:       edgeType,
	}

	return b.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "from_question_id"}, {Name: "to_question_id"}, {Name: "edge_type"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"updated_at": now,
		}),
	}).Create(&edge).Error
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
