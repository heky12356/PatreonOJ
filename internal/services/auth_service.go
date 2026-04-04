package services

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strings"

	"patreonoj/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrAuthInvalidCredentials = errors.New("invalid credentials")
	ErrAuthUserDisabled       = errors.New("user disabled")
	ErrAuthUsernameExists     = errors.New("username already exists")
	ErrAuthUserNotFound       = errors.New("user not found")
	ErrAuthUUIDGeneration     = errors.New("unable to generate unique uuid")
)

type AuthService struct {
	db            *gorm.DB
	uuidGenerator func() string
}

type AuthLoginResult struct {
	UserID      int
	UUID        string
	Username    string
	Permissions []string
}

type AuthRegisterResult struct {
	UUID   string
	UserID int
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{
		db:            db,
		uuidGenerator: func() string { return uuid.New().String() },
	}
}

func (s *AuthService) Login(username, password string) (*AuthLoginResult, error) {
	var user models.User
	encryptedPassword := md5EncodeAuth(password)
	result := s.db.Where("username = ? AND password = ?", username, encryptedPassword).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrAuthInvalidCredentials
		}
		return nil, result.Error
	}

	if user.Status != "" && user.Status != "active" {
		return nil, ErrAuthUserDisabled
	}

	return &AuthLoginResult{
		UserID:      user.Id,
		UUID:        user.UUID,
		Username:    user.Username,
		Permissions: strings.Split(user.Permissions, ","),
	}, nil
}

func (s *AuthService) Register(username, password string) (*AuthRegisterResult, error) {
	var existingUser models.User
	result := s.db.Where("username = ?", username).First(&existingUser)
	if result.Error == nil {
		return nil, ErrAuthUsernameExists
	}
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	userUUID, err := s.generateUniqueUUID()
	if err != nil {
		return nil, err
	}

	newUser := models.User{
		Username: username,
		Password: md5EncodeAuth(password),
		UUID:     userUUID,
	}

	if err := s.db.Create(&newUser).Error; err != nil {
		return nil, err
	}

	return &AuthRegisterResult{
		UUID:   userUUID,
		UserID: newUser.Id,
	}, nil
}

func (s *AuthService) Logout(userID int) error {
	var user models.User
	result := s.db.First(&user, userID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrAuthUserNotFound
		}
		return result.Error
	}

	user.UUID = ""
	return s.db.Save(&user).Error
}

func (s *AuthService) generateUniqueUUID() (string, error) {
	var existingUser models.User
	for i := 0; i < 10; i++ {
		userUUID := s.uuidGenerator()
		result := s.db.Where("uuid = ?", userUUID).First(&existingUser)
		if result.Error == nil {
			continue
		}
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return userUUID, nil
		}
		return "", result.Error
	}
	return "", ErrAuthUUIDGeneration
}

func md5EncodeAuth(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
