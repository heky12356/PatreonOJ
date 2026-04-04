package services

import (
	"errors"

	"patreonoj/internal/models"

	"gorm.io/gorm"
)

var ErrCategoryNotFound = errors.New("category not found")

type CategorySummary struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	ParentID      int    `json:"parent_id"`
	Description   string `json:"description"`
	QuestionCount int    `json:"question_count"`
}

type CategoryCreateInput struct {
	Name        string
	Slug        string
	ParentID    int
	Description string
}

type CategoryUpdateInput struct {
	Name        *string
	Slug        *string
	ParentID    *int
	Description *string
}

type CategoryService struct {
	db *gorm.DB
}

func NewCategoryService(db *gorm.DB) *CategoryService {
	return &CategoryService{db: db}
}

func (s *CategoryService) List() ([]CategorySummary, error) {
	categoryList := make([]models.Category, 0)
	if err := s.db.Preload("Question").Find(&categoryList).Error; err != nil {
		return nil, err
	}

	result := make([]CategorySummary, 0, len(categoryList))
	for _, category := range categoryList {
		result = append(result, CategorySummary{
			Id:            category.Id,
			Name:          category.Name,
			Slug:          category.Slug,
			ParentID:      category.ParentID,
			Description:   category.Description,
			QuestionCount: len(category.Question),
		})
	}
	return result, nil
}

func (s *CategoryService) Create(input CategoryCreateInput) (*models.Category, error) {
	category := models.Category{
		Name:        input.Name,
		Slug:        input.Slug,
		ParentID:    input.ParentID,
		Description: input.Description,
	}
	if err := s.db.Create(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (s *CategoryService) Update(id int, input CategoryUpdateInput) (*models.Category, error) {
	var existing models.Category
	if err := s.db.First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Slug != nil {
		updates["slug"] = *input.Slug
	}
	if input.ParentID != nil {
		updates["parent_id"] = *input.ParentID
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}

	if len(updates) > 0 {
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return nil, err
		}
		if err := s.db.First(&existing, id).Error; err != nil {
			return nil, err
		}
	}

	return &existing, nil
}

func (s *CategoryService) Delete(id int) error {
	var existing models.Category
	if err := s.db.First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCategoryNotFound
		}
		return err
	}
	return s.db.Delete(&existing).Error
}
