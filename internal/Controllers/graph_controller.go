package controllers

import (
	"context"

	"patreonoj/internal/graph"
	"patreonoj/internal/services"
	"gorm.io/gorm"
)

type GraphController struct {
	db              *gorm.DB
	graphService    *graph.QuestionGraphService
	queryService    *services.GraphQueryService
	writeService    *services.GraphWriteService
	analysisService *services.GraphAnalysisService
}

func NewGraphController(db *gorm.DB, graphService *graph.QuestionGraphService, aiService *services.AIService) *GraphController {
	return &GraphController{
		db:              db,
		graphService:    graphService,
		queryService:    services.NewGraphQueryService(graphService),
		writeService:    services.NewGraphWriteService(db, graphService),
		analysisService: services.NewGraphAnalysisService(db, graphService, aiService),
	}
}

func (gc *GraphController) InitGraph(ctx context.Context) error {
	return gc.graphService.InitGraph(ctx, gc.db)
}
