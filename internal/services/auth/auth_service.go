package auth

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

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
	SessionID   string
	ExpiresAt   time.Time
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

	// 检查用户是否有效
	if user.Status != "" && user.Status != "active" {
		return nil, ErrAuthUserDisabled
	}

	// 生成一个session
	str, _ := NewSessionID()

	expireTime := time.Now().Add(7 * 24 * time.Hour)

	session := models.Session{
		SessionId: str,
		UserId:    user.Id,
		ExpiresAt: &expireTime,
	}
	// 将session放入 session数据库里面
	result = s.db.Create(&session)
	if result.Error != nil {
		return nil, result.Error
	}

	return &AuthLoginResult{
		UserID:      user.Id,
		UUID:        user.UUID,
		Username:    user.Username,
		Permissions: strings.Split(user.Permissions, ","),
		SessionID:   str,
		ExpiresAt:   expireTime,
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

func (s *AuthService) Logout(sessionId string) error {
	// 查session表 update一下回收时间
	session := models.Session{}
	result := s.db.Where("session_id = ?", sessionId).First(&session)
	if result.Error != nil {
		return fmt.Errorf("查询sessionId错误 %w", result.Error)
	}
	timeNow := time.Now()
	session.RevokedAt = &timeNow
	result = s.db.Save(&session)
	if result.Error != nil {
		return fmt.Errorf("保存修改错误 %w", result.Error)
	}
	return nil
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

// 生成session id
func NewSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成SessionID失败 %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
