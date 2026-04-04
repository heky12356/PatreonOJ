package services

import (
	"patreonoj/internal/models"

	"gorm.io/gorm"
)

type OverviewService struct {
	db *gorm.DB
}

func NewOverviewService(db *gorm.DB) *OverviewService {
	return &OverviewService{db: db}
}

func (s *OverviewService) GetHomeText() (string, error) {
	view, err := s.getOverview()
	if err != nil {
		return "", err
	}
	return view.HomeText, nil
}

func (s *OverviewService) UpdateHomeText(homeText string) (string, error) {
	view, err := s.getOverview()
	if err != nil {
		return "", err
	}
	view.HomeText = homeText
	if err := s.db.Save(&view).Error; err != nil {
		return "", err
	}
	return view.HomeText, nil
}

func (s *OverviewService) GetAnnouncement() (string, error) {
	view, err := s.getOverview()
	if err != nil {
		return "", err
	}
	return view.Announcement, nil
}

func (s *OverviewService) UpdateAnnouncement(announcement string) (string, error) {
	view, err := s.getOverview()
	if err != nil {
		return "", err
	}
	view.Announcement = announcement
	if err := s.db.Save(&view).Error; err != nil {
		return "", err
	}
	return view.Announcement, nil
}

func (s *OverviewService) getOverview() (models.OjOverView, error) {
	var view models.OjOverView
	if err := s.db.First(&view).Error; err != nil {
		return models.OjOverView{}, err
	}
	return view, nil
}
