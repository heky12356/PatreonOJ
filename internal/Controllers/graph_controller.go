package Controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dachuang/internal/graph"
	"dachuang/internal/models"

	"dachuang/internal/services" // Import services package

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GraphController 图数据库控制器
type GraphController struct {
	db           *gorm.DB
	graphService *graph.QuestionGraphService
	aiService    *services.AIService
}

// NewGraphController 创建新的图控制器
func NewGraphController(db *gorm.DB, graphService *graph.QuestionGraphService, aiService *services.AIService) *GraphController {
	return &GraphController{
		db:           db,
		graphService: graphService,
		aiService:    aiService,
	}
}

// SyncQuestion 同步题目到图数据库
func (gc *GraphController) SyncQuestion(c *gin.Context) {
	questionNumberStr := c.Param("number")
	questionNumber, err := strconv.Atoi(questionNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	var question models.Question
	if err := gc.db.Where("question_number = ?", questionNumber).First(&question).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询题目失败"})
		return
	}

	now := time.Now().UTC()
	questionNode := graph.QuestionNode{
		QuestionNumber: question.QuestionNumber,
		QuestionId:     question.QuestionId,
		Title:          question.Title,
		Difficulty:     question.Difficulty,
		Tags:           question.Tags,
		Status:         question.Status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	ctx := context.Background()
	if err := gc.graphService.CreateOrUpdateQuestion(ctx, questionNode); err != nil {
		log.Printf("同步题目到图数据库失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步题目到图数据库失败"})
		return
	}

	if err := gc.graphService.SyncQuestionSkills(ctx, question.QuestionNumber, question.Tags, now); err != nil {
		log.Printf("同步题目标签到图数据库失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步题目标签失败"})
		return
	}

	if err := gc.graphService.BuildTagSimilarEdgesForQuestion(ctx, question.QuestionNumber, now); err != nil {
		log.Printf("构建同标签题目关系失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "构建同标签题目关系失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "题目、标签及同标签关系同步成功"})
}

// CreateRelation 创建题目关系
func (gc *GraphController) CreateRelation(c *gin.Context) {
	var req struct {
		FromQuestion int                    `json:"from_question" binding:"required"`
		ToQuestion   int                    `json:"to_question" binding:"required"`
		RelationType graph.RelationshipType `json:"relation_type" binding:"required"`
		Weight       float64                `json:"weight"`
		Description  string                 `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证关系类型
	validTypes := map[graph.RelationshipType]bool{
		graph.PREREQUISITE: true,
		graph.NEXT_LEVEL:   true,
		graph.SIMILAR:      true,
		graph.CATEGORY:     true,
	}

	if !validTypes[req.RelationType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的关系类型"})
		return
	}

	// 创建关系
	relation := graph.QuestionRelation{
		FromQuestionNumber: req.FromQuestion,
		ToQuestionNumber:   req.ToQuestion,
		RelationType:       req.RelationType,
		Weight:             req.Weight,
		Description:        req.Description,
		CreatedAt:          time.Now().UTC(), // 使用UTC时间避免时区问题
	}

	ctx := context.Background()
	if err := gc.graphService.CreateRelation(ctx, relation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建关系失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "关系创建成功"})
}

// DeleteRelation 删除题目关系
func (gc *GraphController) DeleteRelation(c *gin.Context) {
	var req struct {
		FromQuestion int                    `json:"from_question" binding:"required"`
		ToQuestion   int                    `json:"to_question" binding:"required"`
		RelationType graph.RelationshipType `json:"relation_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if err := gc.graphService.DeleteRelation(ctx, req.FromQuestion, req.ToQuestion, req.RelationType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除关系失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "关系删除成功"})
}

// GetPrerequisites 获取前置题目
func (gc *GraphController) GetPrerequisites(c *gin.Context) {
	questionNumberStr := c.Param("number")
	questionNumber, err := strconv.Atoi(questionNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	ctx := context.Background()
	prerequisites, err := gc.graphService.GetPrerequisites(ctx, questionNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取前置题目失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question_number": questionNumber,
		"prerequisites":   prerequisites,
	})
}

// GetNextLevelQuestions 获取进阶题目
func (gc *GraphController) GetNextLevelQuestions(c *gin.Context) {
	questionNumberStr := c.Param("number")
	questionNumber, err := strconv.Atoi(questionNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	ctx := context.Background()
	nextQuestions, err := gc.graphService.GetNextLevelQuestions(ctx, questionNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取进阶题目失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question_number": questionNumber,
		"next_questions":  nextQuestions,
	})
}

// FindLearningPath 查找学习路径
func (gc *GraphController) FindLearningPath(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr == "" || endStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供起始题目和目标题目编号"})
		return
	}

	startQuestion, err := strconv.Atoi(startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的起始题目编号"})
		return
	}

	endQuestion, err := strconv.Atoi(endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的目标题目编号"})
		return
	}

	ctx := context.Background()
	path, err := gc.graphService.FindLearningPath(ctx, startQuestion, endQuestion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查找学习路径失败"})
		return
	}

	if path == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "未找到学习路径"})
		return
	}

	c.JSON(http.StatusOK, path)
}

// GetRecommendations 获取推荐题目
func (gc *GraphController) GetRecommendations(c *gin.Context) {
	questionNumberStr := c.Param("number")
	questionNumber, err := strconv.Atoi(questionNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}

	ctx := context.Background()
	recommendations, err := gc.graphService.GetRecommendations(ctx, questionNumber, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取推荐题目失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question_number": questionNumber,
		"recommendations": recommendations,
	})
}

// ListQuestions 获取图节点（题目 + 技能）及边（用于前端可视化）
func (gc *GraphController) ListQuestions(c *gin.Context) {
	ctx := context.Background()

	questions, err := gc.graphService.ListQuestions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取题目节点失败"})
		return
	}
	skills, err := gc.graphService.ListSkills(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取技能节点失败"})
		return
	}

	questionRelations, err := gc.graphService.ListRelations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取题目关系失败"})
		return
	}
	questionSkillRelations, err := gc.graphService.ListQuestionSkillRelations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取题目-技能关系失败"})
		return
	}
	skillRelations, err := gc.graphService.ListSkillRelations(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取技能关系失败"})
		return
	}

	type Edge struct {
		From   string  `json:"from"`
		To     string  `json:"to"`
		Type   string  `json:"type"`
		Weight float64 `json:"weight"`
	}

	questionIdByNumber := make(map[int]string, len(questions))
	for _, q := range questions {
		if q.QuestionNumber == 0 {
			continue
		}
		questionIdByNumber[q.QuestionNumber] = q.QuestionId
	}
	questionKey := func(questionNumber int) string {
		if qid := questionIdByNumber[questionNumber]; qid != "" {
			return "Q:" + qid
		}
		return "Q:" + strconv.Itoa(questionNumber)
	}

	edges := make([]Edge, 0, len(questionRelations)+len(questionSkillRelations)+len(skillRelations))
	for _, r := range questionRelations {
		edges = append(edges, Edge{
			From:   questionKey(r.FromQuestionNumber),
			To:     questionKey(r.ToQuestionNumber),
			Type:   string(r.RelationType),
			Weight: r.Weight,
		})
	}
	for _, r := range questionSkillRelations {
		edges = append(edges, Edge{
			From:   questionKey(r.QuestionNumber),
			To:     "S:" + r.SkillKey,
			Type:   "HAS_SKILL",
			Weight: r.Weight,
		})
	}
	for _, r := range skillRelations {
		edges = append(edges, Edge{
			From:   "S:" + r.FromKey,
			To:     "S:" + r.ToKey,
			Type:   string(r.RelationType),
			Weight: r.Weight,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"questions":                questions,
		"skills":                   skills,
		"question_relations":       questionRelations,
		"question_skill_relations": questionSkillRelations,
		"skill_relations":          skillRelations,
		"edges":                    edges,
		"count":                    len(questions),
		"skill_count":              len(skills),
		"edge_count":               len(edges),
	})
}

// InitGraph 初始化图数据库，同步题目节点和关系
func (gc *GraphController) InitGraph(ctx context.Context) error {
	return gc.graphService.InitGraph(ctx, gc.db)
}

// AnalyzeQuestionRelations 调用AI分析题目关系
func (gc *GraphController) AnalyzeQuestionRelations(c *gin.Context) {
	questionNumberStr := c.Param("number")
	questionNumber, err := strconv.Atoi(questionNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目编号"})
		return
	}

	// 1. 获取目标题目信息
	var targetQuestion models.Question
	if err := gc.db.Where("question_number = ?", questionNumber).First(&targetQuestion).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "题目不存在"})
		return
	}

	// 2. 智能获取候选题目列表
	// 策略：优先同标签题目 + 相近难度，排除自身，限制数量节省 Token
	var candidateQuestions []models.Question

	// 解析目标题目的标签
	targetTags := strings.Split(targetQuestion.Tags, ",")
	for i := range targetTags {
		targetTags[i] = strings.TrimSpace(targetTags[i])
	}

	// 构建 LIKE 查询条件：任一标签匹配
	tagConditions := make([]string, 0, len(targetTags))
	tagArgs := make([]interface{}, 0, len(targetTags))
	for _, tag := range targetTags {
		if tag != "" {
			tagConditions = append(tagConditions, "tags LIKE ?")
			tagArgs = append(tagArgs, "%"+tag+"%")
		}
	}

	query := gc.db.Where("question_number != ?", questionNumber)
	if len(tagConditions) > 0 {
		// 优先匹配同标签题目
		query = query.Where("("+strings.Join(tagConditions, " OR ")+")", tagArgs...)
	}
	// 按难度相近排序（同难度优先）
	orderClause := fmt.Sprintf("CASE WHEN difficulty = '%s' THEN 0 ELSE 1 END", targetQuestion.Difficulty)
	query.Order(orderClause).Limit(30).Find(&candidateQuestions)

	// 如果同标签题目不足，补充其他题目
	if len(candidateQuestions) < 20 {
		var extraQuestions []models.Question
		existingIDs := make([]int, len(candidateQuestions)+1)
		existingIDs[0] = questionNumber
		for i, q := range candidateQuestions {
			existingIDs[i+1] = q.QuestionNumber
		}
		gc.db.Where("question_number NOT IN ?", existingIDs).
			Limit(20 - len(candidateQuestions)).
			Find(&extraQuestions)
		candidateQuestions = append(candidateQuestions, extraQuestions...)
	}

	targetJSON, _ := json.Marshal(map[string]interface{}{
		"id":         targetQuestion.QuestionNumber,
		"title":      targetQuestion.Title,
		"tags":       targetQuestion.Tags,
		"difficulty": targetQuestion.Difficulty,
	})

	candidates := make([]map[string]interface{}, 0, len(candidateQuestions))
	for _, q := range candidateQuestions {
		if q.QuestionNumber == questionNumber {
			continue
		}
		candidates = append(candidates, map[string]interface{}{
			"id":         q.QuestionNumber,
			"title":      q.Title,
			"tags":       q.Tags,
			"difficulty": q.Difficulty,
		})
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := context.Background()
	relations, err := gc.aiService.AnalyzeQuestionRelations(ctx, string(targetJSON), string(candidatesJSON))
	if err != nil {
		log.Printf("AI分析失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI分析失败: " + err.Error()})
		return
	}

	// 4. 保存关系到图数据库
	count := 0
	for _, rel := range relations {
		// 转换关系类型字符串
		var relType graph.RelationshipType
		switch rel.RelationType {
		case "PREREQUISITE":
			relType = graph.PREREQUISITE
		case "SIMILAR":
			relType = graph.SIMILAR
		default:
			continue
		}

		graphRel := graph.QuestionRelation{
			FromQuestionNumber: rel.SourceID,
			ToQuestionNumber:   rel.TargetID, // AI返回的TargetID应该是当前题目ID
			RelationType:       relType,
			Weight:             1.0,
			Description:        rel.Reason,
			CreatedAt:          time.Now().UTC(),
		}

		if rel.TargetID == 0 { // 如果AI没填TargetID，默认是当前题目
			graphRel.ToQuestionNumber = questionNumber
		}

		if err := gc.graphService.CreateRelation(ctx, graphRel); err == nil {
			count++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "分析完成",
		"relations": relations,
		"saved":     count,
	})
}

// AnalyzeSkillTree 调用AI构建技能树
func (gc *GraphController) AnalyzeSkillTree(c *gin.Context) {
	ctx := context.Background()

	// 1. 获取所有技能
	skills, err := gc.graphService.ListSkills(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取技能列表失败"})
		return
	}

	if len(skills) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "暂无技能数据", "relations": []interface{}{}, "saved": 0})
		return
	}

	// 构建 Name -> Key 映射 (AI 返回的是 Name，图数据库需要 Key)
	nameToKey := make(map[string]string, len(skills))
	skillNames := make([]string, 0, len(skills))
	for _, s := range skills {
		nameToKey[s.Name] = s.Key
		skillNames = append(skillNames, s.Name)
	}

	// 2. 调用AI分析技能依赖关系
	relations, err := gc.aiService.AnalyzeSkillTree(ctx, skillNames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI分析失败: " + err.Error()})
		return
	}

	// 3. 保存关系到图数据库
	saved, failed := 0, 0
	for _, rel := range relations {
		fromKey, ok1 := nameToKey[rel.ParentSkill]
		toKey, ok2 := nameToKey[rel.ChildSkill]

		if !ok1 || !ok2 {
			failed++
			continue
		}

		// 创建 SKILL_SUBSUMES 关系 (Parent -> Child)
		if err := gc.graphService.CreateSkillRelation(ctx, fromKey, toKey, graph.SKILL_SUBSUMES, rel.Reason); err != nil {
			log.Printf("创建技能关系失败 %s->%s: %v", fromKey, toKey, err)
			failed++
		} else {
			saved++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "技能树分析完成",
		"relations": relations,
		"saved":     saved,
		"failed":    failed,
	})
}
