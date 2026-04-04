package services

import (
	"errors"
	"strings"

	"dachuang/internal/models"

	"gorm.io/gorm"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserStatusInvalid = errors.New("invalid user status")
	ErrUserRoleNotFound  = errors.New("role not found")
)

type UserUpdateInput struct {
	Nickname    *string
	Email       *string
	AvatarURL   *string
	Status      *string
	Permissions *string
	RoleCodes   []string
}

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) List() ([]models.User, error) {
	userList := make([]models.User, 0)
	if err := s.db.Find(&userList).Error; err != nil {
		return nil, err
	}
	return userList, nil
}

func (s *UserService) GetByUUID(uuid string) (*models.User, error) {
	var user models.User
	if err := s.db.Preload("Roles.Permissions").Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserService) UpdateByUUID(uuid string, input UserUpdateInput) (*models.User, error) {
	var user models.User
	if err := s.db.Preload("Roles").Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	updates := map[string]any{}
	if input.Nickname != nil {
		updates["nickname"] = strings.TrimSpace(*input.Nickname)
	}
	if input.Email != nil {
		updates["email"] = strings.TrimSpace(*input.Email)
	}
	if input.AvatarURL != nil {
		updates["avatar_url"] = strings.TrimSpace(*input.AvatarURL)
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if status != "active" && status != "disabled" {
			return nil, ErrUserStatusInvalid
		}
		updates["status"] = status
	}
	if input.Permissions != nil {
		updates["permissions"] = strings.TrimSpace(*input.Permissions)
	}

	if len(updates) > 0 {
		if err := s.db.Model(&user).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	if len(input.RoleCodes) > 0 {
		normalized := normalizeRoleCodes(input.RoleCodes)
		if len(normalized) > 0 {
			roles := make([]models.Role, 0, len(normalized))
			if err := s.db.Where("name IN ?", normalized).Find(&roles).Error; err != nil {
				return nil, err
			}
			if len(roles) != len(normalized) {
				return nil, ErrUserRoleNotFound
			}
			if err := s.db.Model(&user).Association("Roles").Replace(&roles); err != nil {
				return nil, err
			}
		}
	}

	if err := s.db.Preload("Roles.Permissions").Where("uuid = ?", uuid).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func normalizeRoleCodes(roleCodes []string) []string {
	out := make([]string, 0, len(roleCodes))
	seen := make(map[string]struct{}, len(roleCodes))
	for _, roleCode := range roleCodes {
		v := strings.TrimSpace(roleCode)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
