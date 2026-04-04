package services

import (
	"patreonoj/internal/models"

	"gorm.io/gorm"
)

type RelationService struct {
	db *gorm.DB
}

func NewRelationService(db *gorm.DB) *RelationService {
	return &RelationService{db: db}
}

func (s *RelationService) List() ([]models.Relation, error) {
	relationList := make([]models.Relation, 0)
	if err := s.db.Find(&relationList).Error; err != nil {
		return nil, err
	}
	return relationList, nil
}
