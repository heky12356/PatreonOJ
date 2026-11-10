package middleware

import (
	"net/http"
	"time"

	"patreonoj/internal/models"
	"patreonoj/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func LoadCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err != nil {
			c.Next()
			return
		}

		if sessionID == "" {
			c.Next()
			return
		}

		var session models.Session
		if err := db.Where("session_id = ?", sessionID).First(&session).Error; err != nil {
			c.Set("authError", "invalid_session")
			c.Next()
			return
		}

		if session.RevokedAt != nil || (session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now())) {
			c.Set("authError", "expired_session")
			c.Next()
			return
		}

		var user models.User
		if err := db.Preload("Roles.Permissions").First(&user, session.UserId).Error; err != nil {
			c.Set("authError", "user_not_found")
			c.Next()
			return
		}

		c.Set("currentUser", user)
		c.Set("sessionId", sessionID)
		c.Next()
	}
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, ok := c.Get("currentUser"); ok && user != nil {
			c.Next()
			return
		}

		if authErr, ok := c.Get("authError"); ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": authErr})
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "unauthorized"})
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get("currentUser")
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "unauthorized"})
			return
		}

		if !util.UserInstance.HasPermission(user.(models.User), permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "没有权限"})
			return
		}

		c.Next()
	}
}
