package question

import (
	"errors"
	"gorm.io/gorm"
	"patreonoj/internal/graphcache"
	"patreonoj/internal/models"
	"strings"
)

type RebuildReport struct {
	Applied      bool  `json:"applied"`
	Before       int   `json:"before"`
	After        int64 `json:"after"`
	Protected    int   `json:"protected"`
	Classified   int   `json:"classified_legacy_auto"`
	Automatic    int64 `json:"automatic_after"`
	Unclassified int64 `json:"unclassified_legacy"`
}

// RebuildAutomaticGraph only classifies legacy edges in an explicitly selected import namespace.
// Protected/manual relationships and all question/user/testcase records remain untouched.
func RebuildAutomaticGraph(db *gorm.DB, protected []models.QuestionRelationEdge, legacyPrefix string, apply bool) (RebuildReport, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	report := RebuildReport{Applied: apply}
	dryRun := errors.New("graph dry run")
	key := func(e models.QuestionRelationEdge) string {
		return e.FromQuestionID + "|" + e.ToQuestionID + "|" + string(e.EdgeType)
	}
	protectedKeys := map[string]bool{}
	for _, e := range protected {
		protectedKeys[key(e)] = true
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var questions []models.Question
		if err := tx.Select("id, question_id, tags, difficulty, status").Order("id ASC").Find(&questions).Error; err != nil {
			return err
		}
		byID := map[string]models.Question{}
		for _, q := range questions {
			byID[q.QuestionId] = q
		}
		var edges []models.QuestionRelationEdge
		if err := tx.Find(&edges).Error; err != nil {
			return err
		}
		report.Before = len(edges)
		foundProtected := map[string]bool{}
		for _, e := range edges {
			if protectedKeys[key(e)] {
				foundProtected[key(e)] = true
			}
		}
		if len(foundProtected) != len(protectedKeys) {
			return errors.New("protected manifest contains missing relationships; rebuild aborted")
		}
		manual := make([]models.QuestionRelationEdge, 0)
		for _, e := range edges {
			if protectedKeys[key(e)] || e.Source == "manual" {
				if err := tx.Model(&e).Update("source", "manual").Error; err != nil {
					return err
				}
				manual = append(manual, e)
				report.Protected++
				continue
			}
			if e.Source != "legacy" || legacyPrefix == "" || (!strings.HasPrefix(e.FromQuestionID, legacyPrefix) && !strings.HasPrefix(e.ToQuestionID, legacyPrefix)) {
				continue
			}
			from, ok1 := byID[e.FromQuestionID]
			to, ok2 := byID[e.ToQuestionID]
			if !ok1 || !ok2 {
				continue
			}
			a, b := parseTagSet(from.Tags), parseTagSet(to.Tags)
			inferred := (e.EdgeType == models.QuestionRelationEdgeTypeSimilar && hasIntersection(a, b) && !isStrictSubset(a, b) && !isStrictSubset(b, a)) || ((e.EdgeType == models.QuestionRelationEdgeTypePrerequisite || e.EdgeType == models.QuestionRelationEdgeTypeNextLevel) && isStrictSubset(a, b))
			if inferred {
				if err := tx.Model(&e).Update("source", "auto").Error; err != nil {
					return err
				}
				report.Classified++
			}
		}
		if err := tx.Where("source = ?", "auto").Delete(&models.QuestionRelationEdge{}).Error; err != nil {
			return err
		}
		for _, q := range questions {
			if q.Status == "published" {
				if err := refreshQuestionEdges(tx, q.Id); err != nil {
					return err
				}
			}
		}
		for _, e := range manual {
			var n int64
			if err := tx.Model(&models.QuestionRelationEdge{}).Where("from_question_id = ? AND to_question_id = ? AND edge_type = ? AND source = ?", e.FromQuestionID, e.ToQuestionID, e.EdgeType, "manual").Count(&n).Error; err != nil {
				return err
			}
			if n != 1 {
				return errors.New("protected relationship changed")
			}
		}
		if err := tx.Model(&models.QuestionRelationEdge{}).Count(&report.After).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.QuestionRelationEdge{}).Where("source = ?", "auto").Count(&report.Automatic).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.QuestionRelationEdge{}).Where("source = ?", "legacy").Count(&report.Unclassified).Error; err != nil {
			return err
		}
		if !apply {
			return dryRun
		}
		return nil
	})
	if errors.Is(err, dryRun) {
		err = nil
	}
	if err == nil && apply {
		graphcache.Invalidate()
	}
	return report, err
}
