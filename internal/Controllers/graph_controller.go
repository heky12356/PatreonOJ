package Controllers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"dachuang/internal/graph"
	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GraphController 图数据库控制器
type GraphController struct {
	db           *gorm.DB
	graphService *graph.QuestionGraphService
}

// NewGraphController 创建新的图控制器
func NewGraphController(db *gorm.DB, graphService *graph.QuestionGraphService) *GraphController {
	return &GraphController{
		db:           db,
		graphService: graphService,
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
