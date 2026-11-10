package user

import (
	"errors"
	"strings"

	"patreonoj/internal/models"

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
	RoleCodes   *[]string
}

type UserListFilter struct {
	Keyword  string
	Status   string
	RoleCode string
}

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) List(filter UserListFilter) ([]models.User, error) {
	userList := make([]models.User, 0)
	query := s.db.Model(&models.User{}).Preload("Roles")

	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"username LIKE ? OR nickname LIKE ? OR email LIKE ? OR uuid LIKE ?",
			like,
			like,
			like,
			like,
		)
	}

	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}

	if roleCode := strings.TrimSpace(filter.RoleCode); roleCode != "" {
		query = query.Joins("JOIN user_role ur ON ur.user_id = user.id").
			Joins("JOIN roles r ON r.id = ur.role_id").
			Where("r.name = ?", roleCode).
			Distinct()
	}

	if err := query.Order("user.id DESC").Find(&userList).Error; err != nil {
		return nil, err
	}
	return userList, nil
}

// 废弃了
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

func (s *UserService) GetProfilePublic(id string) (map[string]string, error) {
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}

	req := make(map[string]string)
	req["nickname"] = user.Nickname
	req["email"] = user.Email
	req["avatar_url"] = user.AvatarURL

	return req, nil
}

func (s *UserService) GetProfileById(id string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// 想要废弃，后面通过id来修改
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

	if input.RoleCodes != nil {
		normalized := normalizeRoleCodes(*input.RoleCodes)
		roles := make([]models.Role, 0, len(normalized))
		if len(normalized) > 0 {
			if err := s.db.Where("name IN ?", normalized).Find(&roles).Error; err != nil {
				return nil, err
			}
			if len(roles) != len(normalized) {
				return nil, ErrUserRoleNotFound
			}
		}
		if err := s.db.Model(&user).Association("Roles").Replace(&roles); err != nil {
			return nil, err
		}
	}

	if err := s.db.Preload("Roles.Permissions").Where("uuid = ?", uuid).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) UpdateByID(id string, input UserUpdateInput) (*models.User, error) {
	var user models.User
	if err := s.db.Preload("Roles").Where("id = ?", id).First(&user).Error; err != nil {
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

	if input.RoleCodes != nil {
		normalized := normalizeRoleCodes(*input.RoleCodes)
		roles := make([]models.Role, 0, len(normalized))
		if len(normalized) > 0 {
			if err := s.db.Where("name IN ?", normalized).Find(&roles).Error; err != nil {
				return nil, err
			}
			if len(roles) != len(normalized) {
				return nil, ErrUserRoleNotFound
			}
		}
		if err := s.db.Model(&user).Association("Roles").Replace(&roles); err != nil {
			return nil, err
		}
	}

	if err := s.db.Preload("Roles.Permissions").Where("id = ?", id).First(&user).Error; err != nil {
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
