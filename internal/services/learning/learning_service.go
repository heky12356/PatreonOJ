package learning

import (
	"errors"
	"sort"
	"strings"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var (
	ErrLearningUserNotFound      = errors.New("user not found")
	ErrLearningQuestionNotFound  = errors.New("question not found")
	ErrLearningAmbiguousQuestion = errors.New("question_id is ambiguous for graph relationships")
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

type LearningFeedbackQuestion struct {
	QuestionID string `json:"question_id"`
	QuestionPK int    `json:"question_pk"`
	Title      string `json:"title"`
	Difficulty string `json:"difficulty"`
	Tags       string `json:"tags"`
	Reason     string `json:"reason"`
}

type LearningUserFeedback struct {
	UserUUID         string                     `json:"user_uuid"`
	Stage            string                     `json:"stage"`
	WeakSkills       []string                   `json:"weak_skills"`
	MasteredSkills   []string                   `json:"mastered_skills"`
	NextQuestions    []LearningFeedbackQuestion `json:"next_questions"`
	PositiveFeedback string                     `json:"positive_feedback"`
}

type QuestionGraphNode struct {
	QuestionID string `json:"question_id"`
	QuestionPK int    `json:"question_pk"`
	Title      string `json:"title"`
	Difficulty string `json:"difficulty"`
	Tags       string `json:"tags"`
}

type QuestionGraphEdge struct {
	FromQuestionID string `json:"from_question_id"`
	ToQuestionID   string `json:"to_question_id"`
	EdgeType       string `json:"edge_type"`
}

type QuestionGraphTreeChild struct {
	EdgeType string             `json:"edge_type"`
	Node     *QuestionGraphTree `json:"node"`
}

type QuestionGraphTree struct {
	QuestionID string                   `json:"question_id"`
	QuestionPK int                      `json:"question_pk"`
	Title      string                   `json:"title"`
	Difficulty string                   `json:"difficulty"`
	Tags       string                   `json:"tags"`
	Children   []QuestionGraphTreeChild `json:"children"`
}

type QuestionGraphResult struct {
	Truncated      bool                `json:"truncated"`
	RootQuestionID string              `json:"root_question_id"`
	RootQuestionPK int                 `json:"root_question_pk"`
	Nodes          []QuestionGraphNode `json:"nodes"`
	Edges          []QuestionGraphEdge `json:"edges"`
	Tree           *QuestionGraphTree  `json:"tree"`
}

type LearningService struct {
	db            *gorm.DB
	includeDrafts bool
}

func NewLearningService(db *gorm.DB) *LearningService {
	return &LearningService{db: db}
}

// WithDrafts returns a request-local service; shared controller state is immutable.
func (s *LearningService) WithDrafts(allow bool) *LearningService {
	return &LearningService{db: s.db, includeDrafts: allow}
}

func (s *LearningService) visibleQuestions() *gorm.DB {
	query := s.db.Model(&models.Question{})
	if !s.includeDrafts {
		query = query.Where("status = ?", "published")
	}
	return query
}

func (s *LearningService) uniqueVisibleQuestions() *gorm.DB {
	unique := s.db.Model(&models.Question{}).Select("question_id").Group("question_id").Having("COUNT(*) = 1")
	return s.visibleQuestions().Where("question_id IN (?)", unique)
}

func (s *LearningService) BuildQuestionGraph(questionPK int, depth int) (*QuestionGraphResult, error) {
	return s.BuildQuestionGraphLimited(questionPK, depth, 256, 20000)
}

// BuildQuestionGraphLimited loads question metadata once and uses indexed directional scans.
func (s *LearningService) BuildQuestionGraphLimited(questionPK, depth, maxGraphNodes, maxGraphEdges int) (*QuestionGraphResult, error) {
	depth = min(6, max(1, depth))
	maxGraphNodes = min(256, max(1, maxGraphNodes))
	maxGraphEdges = min(20000, max(1, maxGraphEdges))
	const fields = "id, question_id, title, difficulty, tags, status"
	var root models.Question
	if err := s.visibleQuestions().Select(fields).Where("id = ?", questionPK).First(&root).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLearningQuestionNotFound
		}
		return nil, err
	}
	var duplicates int64
	if err := s.db.Model(&models.Question{}).Where("question_id = ?", root.QuestionId).Count(&duplicates).Error; err != nil {
		return nil, err
	}
	if duplicates != 1 {
		return nil, ErrLearningAmbiguousQuestion
	}
	var metadata []models.Question
	if err := s.uniqueVisibleQuestions().Select(fields).Find(&metadata).Error; err != nil {
		return nil, err
	}
	questionByID := make(map[string]models.Question, len(metadata))
	for _, q := range metadata {
		questionByID[q.QuestionId] = q
	}
	rootID := root.QuestionId
	if _, ok := questionByID[rootID]; !ok {
		return nil, ErrLearningQuestionNotFound
	}
	discovered := map[string]bool{rootID: true}
	frontier := []string{rootID}
	edgeSet := map[string]QuestionGraphEdge{}
	truncated := false
	for level := 0; level < depth && len(frontier) > 0; level++ {
		candidates := make([]models.QuestionRelationEdge, 0)
		for _, direction := range []struct{ column, order string }{{"from_question_id", "from_question_id, edge_type, to_question_id"}, {"to_question_id", "to_question_id, edge_type, from_question_id"}} {
			var batch []models.QuestionRelationEdge
			if err := s.db.Select("from_question_id, to_question_id, edge_type").Where(direction.column+" IN ?", frontier).Order(direction.order).Limit(maxGraphEdges + 1).Find(&batch).Error; err != nil {
				return nil, err
			}
			if len(batch) > maxGraphEdges {
				truncated = true
				batch = batch[:maxGraphEdges]
			}
			candidates = append(candidates, batch...)
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			ai, aj := candidates[i].EdgeType == models.QuestionRelationEdgeTypeSimilar, candidates[j].EdgeType == models.QuestionRelationEdgeTypeSimilar
			if ai != aj {
				return !ai
			}
			if candidates[i].FromQuestionID != candidates[j].FromQuestionID {
				return candidates[i].FromQuestionID < candidates[j].FromQuestionID
			}
			return candidates[i].ToQuestionID < candidates[j].ToQuestionID
		})
		next := map[string]bool{}
		for _, e := range candidates {
			if !e.EdgeType.Valid() {
				continue
			}
			if _, ok := questionByID[e.FromQuestionID]; !ok {
				continue
			}
			if _, ok := questionByID[e.ToQuestionID]; !ok {
				continue
			}
			key := string(e.EdgeType) + "|" + e.FromQuestionID + "|" + e.ToQuestionID
			if _, exists := edgeSet[key]; exists {
				continue
			}
			if len(edgeSet) >= maxGraphEdges {
				truncated = true
				continue
			}
			additional := 0
			for _, id := range []string{e.FromQuestionID, e.ToQuestionID} {
				if !discovered[id] && !next[id] {
					additional++
				}
			}
			if len(discovered)+len(next)+additional > maxGraphNodes {
				truncated = true
				continue
			}
			edgeSet[key] = QuestionGraphEdge{FromQuestionID: e.FromQuestionID, ToQuestionID: e.ToQuestionID, EdgeType: string(e.EdgeType)}
			for _, id := range []string{e.FromQuestionID, e.ToQuestionID} {
				if !discovered[id] {
					next[id] = true
				}
			}
		}
		frontier = mapKeys(next)
		for id := range next {
			discovered[id] = true
		}
	}
	nodes := make([]QuestionGraphNode, 0, len(discovered))
	for id := range discovered {
		q := questionByID[id]
		nodes = append(nodes, QuestionGraphNode{QuestionID: q.QuestionId, QuestionPK: q.Id, Title: q.Title, Difficulty: q.Difficulty, Tags: q.Tags})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].QuestionPK < nodes[j].QuestionPK })
	edges := make([]QuestionGraphEdge, 0, len(edgeSet))
	for _, e := range edgeSet {
		edges = append(edges, e)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromQuestionID != edges[j].FromQuestionID {
			return edges[i].FromQuestionID < edges[j].FromQuestionID
		}
		if edges[i].ToQuestionID != edges[j].ToQuestionID {
			return edges[i].ToQuestionID < edges[j].ToQuestionID
		}
		return edges[i].EdgeType < edges[j].EdgeType
	})
	return &QuestionGraphResult{RootQuestionID: rootID, RootQuestionPK: root.Id, Nodes: nodes, Edges: edges, Tree: buildGraphTree(rootID, questionByID, edges), Truncated: truncated}, nil
}

func buildGraphTree(rootID string, questionByID map[string]models.Question, edges []QuestionGraphEdge) *QuestionGraphTree {
	makeNode := func(id string) *QuestionGraphTree {
		q, ok := questionByID[id]
		if !ok {
			return nil
		}
		return &QuestionGraphTree{QuestionID: q.QuestionId, QuestionPK: q.Id, Title: q.Title, Difficulty: q.Difficulty, Tags: q.Tags, Children: []QuestionGraphTreeChild{}}
	}
	root := makeNode(rootID)
	if root == nil {
		return nil
	}
	adj := map[string][]QuestionGraphEdge{}
	for _, edge := range edges {
		adj[edge.FromQuestionID] = append(adj[edge.FromQuestionID], edge)
	}
	visited := map[string]bool{rootID: true}
	queue := []*QuestionGraphTree{root}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, edge := range adj[node.QuestionID] {
			if visited[edge.ToQuestionID] {
				continue
			}
			child := makeNode(edge.ToQuestionID)
			if child == nil {
				continue
			}
			visited[edge.ToQuestionID] = true
			node.Children = append(node.Children, QuestionGraphTreeChild{EdgeType: edge.EdgeType, Node: child})
			queue = append(queue, child)
		}
	}
	return root
}

func mapKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type recCandidate struct {
	questionID string
	edgeType   string
	reason     string
}

const (
	feedbackWeakThreshold     = 0.5
	feedbackMasteredThreshold = 0.8
)

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
	if err := s.visibleQuestions().Where("id = ?", questionPK).First(&question).Error; err != nil {
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

	recommendations, exists, err := s.buildRecommendations(candidates, currentQuestionID, solvedQuestionIDs, limit)
	if err != nil {
		return nil, err
	}
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

func (s *LearningService) BuildUserFeedback(userUUID string, limit int) (*LearningUserFeedback, error) {
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

	masteries := make([]models.UserSkillMastery, 0)
	if err := s.db.Where("user_id = ?", userUUID).
		Order("mastery ASC, skill_key ASC").
		Find(&masteries).Error; err != nil {
		return nil, err
	}

	weakSkills, masteredSkills := classifyFeedbackSkills(masteries)
	solvedQuestionIDs, err := s.listSolvedQuestionIDs(userUUID)
	if err != nil {
		return nil, err
	}

	targetSkills := append([]string{}, weakSkills...)
	if len(targetSkills) == 0 {
		targetSkills = append(targetSkills, masteredSkills...)
	}

	exists := make(map[string]struct{})
	nextQuestions, err := s.collectFeedbackQuestionsBySkills(targetSkills, solvedQuestionIDs, exists, limit)
	if err != nil {
		return nil, err
	}
	if len(nextQuestions) < limit {
		fallback, err := s.collectFeedbackQuestionsBySkills(nil, solvedQuestionIDs, exists, limit-len(nextQuestions))
		if err != nil {
			return nil, err
		}
		nextQuestions = append(nextQuestions, fallback...)
	}

	return &LearningUserFeedback{
		UserUUID:         userUUID,
		Stage:            buildFeedbackStage(weakSkills, masteredSkills, len(masteries)),
		WeakSkills:       weakSkills,
		MasteredSkills:   masteredSkills,
		NextQuestions:    nextQuestions,
		PositiveFeedback: buildPositiveFeedback(weakSkills, masteredSkills, len(nextQuestions)),
	}, nil
}

func classifyFeedbackSkills(masteries []models.UserSkillMastery) ([]string, []string) {
	weakSkills := make([]string, 0)
	masteredSkills := make([]string, 0)
	for _, mastery := range masteries {
		skill := strings.TrimSpace(mastery.SkillKey)
		if skill == "" {
			continue
		}
		switch {
		case mastery.Mastery < feedbackWeakThreshold:
			weakSkills = append(weakSkills, skill)
		case mastery.Mastery >= feedbackMasteredThreshold:
			masteredSkills = append(masteredSkills, skill)
		}
	}
	sort.Strings(weakSkills)
	sort.Strings(masteredSkills)
	return weakSkills, masteredSkills
}

func (s *LearningService) collectFeedbackQuestionsBySkills(targetSkills []string, solved map[string]struct{}, exists map[string]struct{}, limit int) ([]LearningFeedbackQuestion, error) {
	if limit <= 0 {
		return []LearningFeedbackQuestion{}, nil
	}

	query := s.db.Model(&models.Question{}).Where("status = ?", "published")
	if len(targetSkills) > 0 {
		conds := make([]string, 0, len(targetSkills))
		args := make([]interface{}, 0, len(targetSkills))
		for _, skill := range targetSkills {
			skill = strings.ToLower(strings.TrimSpace(skill))
			if skill == "" {
				continue
			}
			conds = append(conds, "LOWER(tags) LIKE ?")
			args = append(args, "%"+skill+"%")
		}
		if len(conds) > 0 {
			query = query.Where(strings.Join(conds, " OR "), args...)
		}
	}

	candidates := make([]models.Question, 0)
	if err := query.Order("id DESC").Limit(limit * 4).Find(&candidates).Error; err != nil {
		return nil, err
	}

	out := make([]LearningFeedbackQuestion, 0, limit)
	for _, q := range candidates {
		if len(out) >= limit {
			break
		}
		questionID := strings.TrimSpace(q.QuestionId)
		if questionID == "" {
			continue
		}
		if _, ok := solved[questionID]; ok {
			continue
		}
		if _, ok := exists[questionID]; ok {
			continue
		}
		exists[questionID] = struct{}{}

		out = append(out, LearningFeedbackQuestion{
			QuestionID: questionID,
			QuestionPK: q.Id,
			Title:      q.Title,
			Difficulty: q.Difficulty,
			Tags:       q.Tags,
			Reason:     buildFeedbackQuestionReason(q.Tags, targetSkills),
		})
	}
	return out, nil
}

func buildFeedbackQuestionReason(tags string, targetSkills []string) string {
	if len(targetSkills) == 0 {
		return "先完成一题未掌握记录较少的训练题，积累后续推荐依据。"
	}
	for _, skill := range targetSkills {
		skill = strings.TrimSpace(skill)
		if skill == "" {
			continue
		}
		if strings.Contains(strings.ToLower(tags), strings.ToLower(skill)) {
			return "围绕薄弱知识点「" + skill + "」进行针对性练习。"
		}
	}
	return "与当前能力画像相关，适合作为下一步训练题。"
}

func buildFeedbackStage(weakSkills []string, masteredSkills []string, masteryCount int) string {
	switch {
	case masteryCount == 0:
		return "起步训练阶段"
	case len(weakSkills) > 0:
		return formatSkillList(weakSkills, 2) + " 强化阶段"
	case len(masteredSkills) > 0:
		return formatSkillList(masteredSkills, 2) + " 进阶阶段"
	default:
		return "稳定提升阶段"
	}
}

func buildPositiveFeedback(weakSkills []string, masteredSkills []string, questionCount int) string {
	switch {
	case len(weakSkills) > 0 && len(masteredSkills) > 0:
		return "你已经掌握 " + formatSkillList(masteredSkills, 3) + "，接下来建议重点补强 " + formatSkillList(weakSkills, 3) + "。"
	case len(weakSkills) > 0:
		return "当前最适合围绕 " + formatSkillList(weakSkills, 3) + " 做针对性训练，先把基础链路补稳。"
	case len(masteredSkills) > 0:
		return "你在 " + formatSkillList(masteredSkills, 3) + " 上表现稳定，可以尝试更高一层的综合题。"
	case questionCount > 0:
		return "先完成推荐题，系统会根据新的提交记录逐步形成更准确的能力画像。"
	default:
		return "当前可用学习数据较少，建议先完成几道基础题来启动个性化推荐。"
	}
}

func formatSkillList(skills []string, limit int) string {
	if len(skills) == 0 {
		return "基础知识点"
	}
	if limit <= 0 || limit > len(skills) {
		limit = len(skills)
	}
	return strings.Join(skills[:limit], "、")
}

func classifyLearningState(attempts, passCount int64, passRate float64) (LearningState, string) {
	switch {
	case attempts == 0:
		return LearningStateNew, "还没有尝试记录，建议先从相似题开始。"
	case passCount > 0 || passRate >= 0.6:
		return LearningStateMastered, "当前题目基本掌握，可以尝试进阶题或变式题。"
	case attempts >= 3 && passCount == 0:
		return LearningStateChallenged, "当前题目卡住了，建议先补前置题。"
	default:
		return LearningStateLearning, "正在学习阶段，继续练习同类题巩固。"
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
		reason := "先补齐该题依赖的前置知识点。"
		if e.EdgeType == models.QuestionRelationEdgeTypeNextLevel {
			reason = "先做一道更基础的过渡题。"
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
		reason := "继续向下一难度层级进阶。"
		if e.EdgeType == models.QuestionRelationEdgeTypeSimilar {
			reason = "通过同层变式题进一步巩固。"
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
			reason:     "练习相似题以强化当前知识点。",
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
			reason:     "练习相似题以强化当前知识点。",
		})
	}

	return out, nil
}

func (s *LearningService) buildRecommendations(candidates []recCandidate, currentQuestionID string, solved map[string]struct{}, limit int) ([]LearningRecommendation, map[string]struct{}, error) {
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
		if err := s.uniqueVisibleQuestions().Where("question_id IN ?", ids).Find(&questions).Error; err != nil {
			return nil, nil, err
		}
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
	return recs, exists, nil
}

func (s *LearningService) buildTagFallbackRecommendations(current models.Question, solved map[string]struct{}, exists map[string]struct{}, limit int) ([]LearningRecommendation, error) {
	if limit <= 0 {
		return []LearningRecommendation{}, nil
	}

	tokens := splitTags(current.Tags)
	if len(tokens) == 0 {
		return []LearningRecommendation{}, nil
	}

	query := s.uniqueVisibleQuestions().Where("id <> ?", current.Id)
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
			Reason:     "按标签相似度扩展练习范围。",
		})
	}

	return out, nil
}

func splitTags(tags string) []string {
	fields := strings.FieldsFunc(tags, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '|', '/', '\\', ' ':
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
