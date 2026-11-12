package question

import (
	"gorm.io/gorm/clause"
	"log"
	"sort"
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

// NewAsyncQuestionEdgeBuilder keeps the legacy constructor signature.
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
	mutationMu.Lock()
	defer mutationMu.Unlock()
	return b.db.Transaction(func(tx *gorm.DB) error { return refreshQuestionEdges(tx, questionNumber) })
}

func refreshQuestionEdges(db *gorm.DB, questionNumber int) error {
	var current models.Question
	if err := db.Select("id, question_id, tags, difficulty, status").Where("id = ?", questionNumber).First(&current).Error; err != nil {
		return err
	}
	// Rebuilding inferred relations must never erase manual or unclassified edges.
	if err := db.Where("source = ? AND (from_question_id = ? OR to_question_id = ?)", "auto", current.QuestionId, current.QuestionId).Delete(&models.QuestionRelationEdge{}).Error; err != nil {
		return err
	}
	if current.Status != "published" {
		return nil
	}
	var others []models.Question
	if err := db.Select("id, question_id, tags, difficulty, status").Find(&others).Error; err != nil {
		return err
	}
	counts := map[string]int{}
	for _, q := range others {
		counts[q.QuestionId]++
	}
	if counts[current.QuestionId] != 1 {
		return nil
	}
	tags := parseTagSet(current.Tags)
	if len(tags) == 0 {
		return nil
	}
	var stored []models.QuestionRelationEdge
	if err := db.Select("from_question_id, to_question_id, edge_type, source").Find(&stored).Error; err != nil {
		return err
	}
	exists := map[string]bool{}
	similar, outgoing, incoming := map[string]int{}, map[string]int{}, map[string]int{}
	key := func(from, to string, kind models.QuestionRelationEdgeType) string {
		return from + "|" + to + "|" + string(kind)
	}
	for _, e := range stored {
		exists[key(e.FromQuestionID, e.ToQuestionID, e.EdgeType)] = true
		if e.Source != "auto" {
			continue
		}
		if e.EdgeType == models.QuestionRelationEdgeTypeSimilar {
			similar[e.FromQuestionID]++
		}
		if e.EdgeType == models.QuestionRelationEdgeTypeNextLevel {
			outgoing[e.FromQuestionID]++
			incoming[e.ToQuestionID]++
		}
	}
	type candidate struct {
		q        models.Question
		kind     string
		score    float64
		distance int
	}
	candidates := make([]candidate, 0)
	for _, q := range others {
		if q.Id == current.Id || q.Status != "published" || counts[q.QuestionId] != 1 {
			continue
		}
		other := parseTagSet(q.Tags)
		if len(other) == 0 {
			continue
		}
		overlap := 0
		for tag := range tags {
			if _, ok := other[tag]; ok {
				overlap++
			}
		}
		if overlap == 0 {
			continue
		}
		kind := "similar"
		if isStrictSubset(other, tags) {
			if difficultyRank(q.Difficulty) > difficultyRank(current.Difficulty) {
				continue
			}
			kind = "prior"
		} else if isStrictSubset(tags, other) {
			if difficultyRank(current.Difficulty) > difficultyRank(q.Difficulty) {
				continue
			}
			kind = "next"
		}
		score := float64(overlap)/float64(len(tags)+len(other)-overlap) - float64(abs(difficultyRank(current.Difficulty)-difficultyRank(q.Difficulty)))*0.03
		candidates = append(candidates, candidate{q: q, kind: kind, score: score, distance: abs(q.Id - current.Id)})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].q.Id < candidates[j].q.Id
	})
	chosen := map[string]int{}
	rows := make([]models.QuestionRelationEdge, 0, 24)
	add := func(from, to string, kind models.QuestionRelationEdgeType) bool {
		k := key(from, to, kind)
		if exists[k] {
			return false
		}
		exists[k] = true
		rows = append(rows, models.QuestionRelationEdge{FromQuestionID: from, ToQuestionID: to, EdgeType: kind, Source: "auto"})
		return true
	}
	for _, candidate := range candidates {
		if chosen[candidate.kind] >= 4 {
			continue
		}
		other := candidate.q.QuestionId
		if candidate.kind == "similar" {
			if similar[current.QuestionId] >= 8 || similar[other] >= 8 {
				continue
			}
			a := add(current.QuestionId, other, models.QuestionRelationEdgeTypeSimilar)
			b := add(other, current.QuestionId, models.QuestionRelationEdgeTypeSimilar)
			if a {
				similar[current.QuestionId]++
			}
			if b {
				similar[other]++
			}
			if a || b {
				chosen[candidate.kind]++
			}
		} else {
			from, to := current.QuestionId, other
			if candidate.kind == "prior" {
				from, to = other, current.QuestionId
			}
			if outgoing[from] >= 8 || incoming[to] >= 4 {
				continue
			}
			a := add(from, to, models.QuestionRelationEdgeTypePrerequisite)
			b := add(from, to, models.QuestionRelationEdgeTypeNextLevel)
			if b {
				outgoing[from]++
				incoming[to]++
			}
			if a || b {
				chosen[candidate.kind]++
			}
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 100).Error
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func difficultyRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "简单", "easy":
		return 1
	case "困难", "hard":
		return 3
	default:
		return 2
	}
}

func deleteQuestionEdges(db *gorm.DB, questionID string) error {
	return db.Where("from_question_id = ? OR to_question_id = ?", questionID, questionID).Delete(&models.QuestionRelationEdge{}).Error
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
