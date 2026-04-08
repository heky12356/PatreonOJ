package learning

import (
	"errors"
	"strings"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var (
	ErrLearningUserNotFound     = errors.New("user not found")
	ErrLearningQuestionNotFound = errors.New("question not found")
)

type LearningState string

const (
	LearningStateNew        LearningState = "new"
	LearningStateLearning   LearningState = "learning"
	LearningStateChallenged LearningState = "challenged"
	LearningStateMastered   LearningState = "mastered"
)

type LearningRecommendation struct {
	QuestionID string `json:"question_id"`
	QuestionPK int    `json:"question_pk"`
	Title      string `json:"title"`
	Difficulty string `json:"difficulty"`
	Tags       string `json:"tags"`
	EdgeType   string `json:"edge_type"`
	Reason     string `json:"reason"`
}

type LearningRecommendationResult struct {
	UserUUID        string                   `json:"user_uuid"`
	QuestionID      string                   `json:"question_id"`
	QuestionPK      int                      `json:"question_pk"`
	Attempts        int64                    `json:"attempts"`
	PassCount       int64                    `json:"pass_count"`
	PassRate        float64                  `json:"pass_rate"`
	State           LearningState            `json:"state"`
	Message         string                   `json:"message"`
	Recommendations []LearningRecommendation `json:"recommendations"`
}

type LearningService struct {
	db *gorm.DB
}

func NewLearningService(db *gorm.DB) *LearningService {
	return &LearningService{db: db}
}

type recCandidate struct {
	questionID string
	edgeType   string
	reason     string
}

func (s *LearningService) RecommendNextQuestions(userUUID string, questionPK int, limit int) (*LearningRecommendationResult, error) {
	userUUID = strings.TrimSpace(userUUID)
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	var user models.User
	if err := s.db.Where("uuid = ?", userUUID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLearningUserNotFound
		}
		return nil, err
	}

	var question models.Question
	if err := s.db.Where("id = ?", questionPK).First(&question).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLearningQuestionNotFound
		}
		return nil, err
	}

	var attempts int64
	if err := s.db.Model(&models.Submission{}).
		Where("user_id = ? AND question_id = ?", userUUID, question.Id).
		Count(&attempts).Error; err != nil {
		return nil, err
	}

	var passCount int64
	if err := s.db.Model(&models.UserSolvedQuestion{}).
		Where("user_uuid = ? AND question_id = ?", userUUID, question.Id).
		Count(&passCount).Error; err != nil {
		return nil, err
	}

	passRate := 0.0
	if attempts > 0 {
		passRate = float64(passCount) / float64(attempts)
	}

	state, msg := classifyLearningState(attempts, passCount, passRate)

	solvedQuestionIDs, err := s.listSolvedQuestionIDs(userUUID)
	if err != nil {
		return nil, err
	}

	currentQuestionID := strings.TrimSpace(question.QuestionId)
	candidates, err := s.collectCandidates(state, currentQuestionID, limit*3)
	if err != nil {
		return nil, err
	}

	recommendations, exists := s.buildRecommendations(candidates, currentQuestionID, solvedQuestionIDs, limit)
	if len(recommendations) < limit {
		need := limit - len(recommendations)
		fallback, err := s.buildTagFallbackRecommendations(question, solvedQuestionIDs, exists, need)
		if err != nil {
			return nil, err
		}
		recommendations = append(recommendations, fallback...)
	}

	return &LearningRecommendationResult{
		UserUUID:        userUUID,
		QuestionID:      question.QuestionId,
		QuestionPK:      question.Id,
		Attempts:        attempts,
		PassCount:       passCount,
		PassRate:        passRate,
		State:           state,
		Message:         msg,
		Recommendations: recommendations,
	}, nil
}

func classifyLearningState(attempts, passCount int64, passRate float64) (LearningState, string) {
	switch {
	case attempts == 0:
		return LearningStateNew, "No attempts yet. Start with similar problems."
	case passCount > 0 || passRate >= 0.6:
		return LearningStateMastered, "Current problem is mostly mastered. Try harder or variant problems."
	case attempts >= 3 && passCount == 0:
		return LearningStateChallenged, "You seem blocked. Try prerequisite problems first."
	default:
		return LearningStateLearning, "You are in progress. Keep practicing with similar problems."
	}
}

func (s *LearningService) listSolvedQuestionIDs(userUUID string) (map[string]struct{}, error) {
	type row struct {
		QuestionID string `gorm:"column:question_id"`
	}
	rows := make([]row, 0)
	if err := s.db.Table("user_solved_question usq").
		Select("q.question_id").
		Joins("JOIN question q ON q.id = usq.question_id").
		Where("usq.user_uuid = ?", userUUID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		id := strings.TrimSpace(r.QuestionID)
		if id == "" {
			continue
		}
		out[id] = struct{}{}
	}
	return out, nil
}

func (s *LearningService) collectCandidates(state LearningState, currentQuestionID string, limit int) ([]recCandidate, error) {
	switch state {
	case LearningStateChallenged:
		return s.collectPrerequisiteCandidates(currentQuestionID, limit)
	case LearningStateMastered:
		return s.collectMasteredCandidates(currentQuestionID, limit)
	default:
		return s.collectSimilarCandidates(currentQuestionID, limit)
	}
}

func (s *LearningService) collectPrerequisiteCandidates(currentQuestionID string, limit int) ([]recCandidate, error) {
	edges := make([]models.QuestionRelationEdge, 0)
	if err := s.db.Where("to_question_id = ? AND edge_type IN ?", currentQuestionID, []models.QuestionRelationEdgeType{
		models.QuestionRelationEdgeTypePrerequisite, models.QuestionRelationEdgeTypeNextLevel,
	}).Order("updated_at DESC, id DESC").Limit(limit).Find(&edges).Error; err != nil {
		return nil, err
	}

	out := make([]recCandidate, 0, len(edges))
	for _, e := range edges {
		reason := "Fill prerequisite knowledge points."
		if e.EdgeType == models.QuestionRelationEdgeTypeNextLevel {
			reason = "Try a simpler stepping-stone problem first."
		}
		out = append(out, recCandidate{
			questionID: e.FromQuestionID,
			edgeType:   string(e.EdgeType),
			reason:     reason,
		})
	}
	return out, nil
}

func (s *LearningService) collectMasteredCandidates(currentQuestionID string, limit int) ([]recCandidate, error) {
	edges := make([]models.QuestionRelationEdge, 0)
	if err := s.db.Where("from_question_id = ? AND edge_type IN ?", currentQuestionID, []models.QuestionRelationEdgeType{
		models.QuestionRelationEdgeTypeNextLevel, models.QuestionRelationEdgeTypeSimilar,
	}).Order("updated_at DESC, id DESC").Limit(limit).Find(&edges).Error; err != nil {
		return nil, err
	}

	out := make([]recCandidate, 0, len(edges))
	for _, e := range edges {
		reason := "Keep moving to the next level."
		if e.EdgeType == models.QuestionRelationEdgeTypeSimilar {
			reason = "Consolidate with same-level variants."
		}
		out = append(out, recCandidate{
			questionID: e.ToQuestionID,
			edgeType:   string(e.EdgeType),
			reason:     reason,
		})
	}
	return out, nil
}

func (s *LearningService) collectSimilarCandidates(currentQuestionID string, limit int) ([]recCandidate, error) {
	out := make([]recCandidate, 0, limit*2)

	edgesOut := make([]models.QuestionRelationEdge, 0)
	if err := s.db.Where("from_question_id = ? AND edge_type = ?", currentQuestionID, models.QuestionRelationEdgeTypeSimilar).
		Order("updated_at DESC, id DESC").Limit(limit).Find(&edgesOut).Error; err != nil {
		return nil, err
	}
	for _, e := range edgesOut {
		out = append(out, recCandidate{
			questionID: e.ToQuestionID,
			edgeType:   string(e.EdgeType),
			reason:     "Practice similar problems for reinforcement.",
		})
	}

	edgesIn := make([]models.QuestionRelationEdge, 0)
	if err := s.db.Where("to_question_id = ? AND edge_type = ?", currentQuestionID, models.QuestionRelationEdgeTypeSimilar).
		Order("updated_at DESC, id DESC").Limit(limit).Find(&edgesIn).Error; err != nil {
		return nil, err
	}
	for _, e := range edgesIn {
		out = append(out, recCandidate{
			questionID: e.FromQuestionID,
			edgeType:   string(e.EdgeType),
			reason:     "Practice similar problems for reinforcement.",
		})
	}

	return out, nil
}

func (s *LearningService) buildRecommendations(candidates []recCandidate, currentQuestionID string, solved map[string]struct{}, limit int) ([]LearningRecommendation, map[string]struct{}) {
	ids := make([]string, 0, len(candidates))
	for _, c := range candidates {
		id := strings.TrimSpace(c.questionID)
		if id == "" || id == currentQuestionID {
			continue
		}
		ids = append(ids, id)
	}

	questions := make([]models.Question, 0)
	if len(ids) > 0 {
		_ = s.db.Where("question_id IN ?", ids).Find(&questions).Error
	}
	questionByID := make(map[string]models.Question, len(questions))
	for _, q := range questions {
		questionByID[q.QuestionId] = q
	}

	recs := make([]LearningRecommendation, 0, limit)
	exists := make(map[string]struct{})
	for _, c := range candidates {
		if len(recs) >= limit {
			break
		}
		id := strings.TrimSpace(c.questionID)
		if id == "" || id == currentQuestionID {
			continue
		}
		if _, ok := solved[id]; ok {
			continue
		}
		if _, ok := exists[id]; ok {
			continue
		}
		q, ok := questionByID[id]
		if !ok {
			continue
		}
		exists[id] = struct{}{}
		recs = append(recs, LearningRecommendation{
			QuestionID: q.QuestionId,
			QuestionPK: q.Id,
			Title:      q.Title,
			Difficulty: q.Difficulty,
			Tags:       q.Tags,
			EdgeType:   c.edgeType,
			Reason:     c.reason,
		})
	}
	return recs, exists
}

func (s *LearningService) buildTagFallbackRecommendations(current models.Question, solved map[string]struct{}, exists map[string]struct{}, limit int) ([]LearningRecommendation, error) {
	if limit <= 0 {
		return []LearningRecommendation{}, nil
	}

	tokens := splitTags(current.Tags)
	if len(tokens) == 0 {
		return []LearningRecommendation{}, nil
	}

	query := s.db.Model(&models.Question{}).Where("id <> ?", current.Id)
	conds := make([]string, 0, len(tokens))
	args := make([]interface{}, 0, len(tokens))
	for _, t := range tokens {
		conds = append(conds, "tags LIKE ?")
		args = append(args, "%"+t+"%")
	}
	query = query.Where(strings.Join(conds, " OR "), args...)

	candidates := make([]models.Question, 0)
	if err := query.Order("id DESC").Limit(limit * 3).Find(&candidates).Error; err != nil {
		return nil, err
	}

	out := make([]LearningRecommendation, 0, limit)
	for _, q := range candidates {
		if len(out) >= limit {
			break
		}
		if _, ok := solved[q.QuestionId]; ok {
			continue
		}
		if _, ok := exists[q.QuestionId]; ok {
			continue
		}
		exists[q.QuestionId] = struct{}{}
		out = append(out, LearningRecommendation{
			QuestionID: q.QuestionId,
			QuestionPK: q.Id,
			Title:      q.Title,
			Difficulty: q.Difficulty,
			Tags:       q.Tags,
			EdgeType:   "TAG_MATCH",
			Reason:     "Expand practice by tag similarity.",
		})
	}

	return out, nil
}

func splitTags(tags string) []string {
	fields := strings.FieldsFunc(tags, func(r rune) bool {
		switch r {
		case ',', ';', '|', '/', '\\', ' ':
			return true
		default:
			return false
		}
	})

	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		token := strings.ToLower(strings.TrimSpace(f))
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}
