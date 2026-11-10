package user

import (
	"errors"
	"strconv"
	"strings"
	"time"

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
	query := s.db.Model(&models.User{}).Preload("Roles.Permissions")

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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	req := make(map[string]string)
	req["nickname"] = user.Nickname
	req["avatar_url"] = user.AvatarURL

	return req, nil
}

func (s *UserService) GetProfileById(id string) (*models.User, error) {
	var user models.User
	if err := s.db.Preload("Roles.Permissions").Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByIdentifier keeps numeric IDs compatible while supporting the UUIDs used
// by profiles and the administration UI. Authorization belongs to the caller.
func (s *UserService) GetByIdentifier(identifier string) (*models.User, error) {
	identifier = strings.TrimSpace(identifier)
	if id, err := strconv.Atoi(identifier); err == nil && id > 0 {
		return s.GetProfileById(strconv.Itoa(id))
	}
	return s.GetByUUID(identifier)
}

// UpdateByUUID is retained for existing service callers.
func (s *UserService) UpdateByUUID(uuid string, input UserUpdateInput) (*models.User, error) {
	return s.update("uuid = ?", uuid, input)
}

func (s *UserService) UpdateByID(id string, input UserUpdateInput) (*models.User, error) {
	return s.update("id = ?", id, input)
}

func (s *UserService) update(condition string, value interface{}, input UserUpdateInput) (*models.User, error) {
	var user models.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Preload("Roles").Where(condition, value).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return err
		}
		updates := map[string]interface{}{}
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
				return ErrUserStatusInvalid
			}
			updates["status"] = status
		}
		if input.Permissions != nil {
			updates["permissions"] = strings.TrimSpace(*input.Permissions)
		}
		var roles []models.Role
		if input.RoleCodes != nil {
			normalized := normalizeRoleCodes(*input.RoleCodes)
			roles = make([]models.Role, 0, len(normalized))
			if len(normalized) > 0 {
				if err := tx.Where("name IN ?", normalized).Find(&roles).Error; err != nil {
					return err
				}
				if len(roles) != len(normalized) {
					return ErrUserRoleNotFound
				}
			}
		}
		if len(updates) > 0 {
			if err := tx.Model(&user).Updates(updates).Error; err != nil {
				return err
			}
		}
		if updates["status"] == "disabled" {
			if err := tx.Model(&models.Session{}).Where("user_id = ? AND revoked_at IS NULL", user.Id).
				Update("revoked_at", time.Now()).Error; err != nil {
				return err
			}
		}
		if input.RoleCodes != nil {
			if err := tx.Model(&user).Association("Roles").Replace(&roles); err != nil {
				return err
			}
		}
		return tx.Preload("Roles.Permissions").First(&user, user.Id).Error
	})
	if err != nil {
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
