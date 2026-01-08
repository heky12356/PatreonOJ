package Controllers

import (
	"dachuang/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OjOverViewController struct {
	db *gorm.DB
}

func NewOjOverViewController(db *gorm.DB) *OjOverViewController {
	return &OjOverViewController{db: db}
}

// GetHomeText 获取OJ首页文本
func (oc *OjOverViewController) GetHomeText(c *gin.Context) {
	var view models.OjOverView
	if err := oc.db.First(&view).Error; err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"home_text": view.HomeText,
	})
}

// UpdateHomeText 更新OJ首页文本
func (oc *OjOverViewController) UpdateHomeText(c *gin.Context) {
	var view models.OjOverView
	if err := oc.db.First(&view).Error; err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	var input struct {
		HomeText string `json:"home_text"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	view.HomeText = input.HomeText
	if err := oc.db.Save(&view).Error; err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"code":      200,
		"home_text": view.HomeText,
	})
}

// GetAnnouncement 获取OJ公告
func (oc *OjOverViewController) GetAnnouncement(c *gin.Context) {
	var view models.OjOverView
	if err := oc.db.First(&view).Error; err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"announcement": view.Announcement,
	})
}

// UpdateAnnouncement 更新OJ公告
func (oc *OjOverViewController) UpdateAnnouncement(c *gin.Context) {
	var view models.OjOverView
	if err := oc.db.First(&view).Error; err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	var input struct {
		Announcement string `json:"announcement"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	view.Announcement = input.Announcement
	if err := oc.db.Save(&view).Error; err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"code":         200,
		"announcement": view.Announcement,
	})
}
