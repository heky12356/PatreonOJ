package services

import (
	"dachuang/internal/models"

	"gorm.io/gorm"
)

type NodeService struct {
	db *gorm.DB
}

func NewNodeService(db *gorm.DB) *NodeService {
	return &NodeService{db: db}
}

func (s *NodeService) List() ([]models.Node, error) {
	nodeList := make([]models.Node, 0)
	if err := s.db.Find(&nodeList).Error; err != nil {
		return nil, err
	}
	return nodeList, nil
}
