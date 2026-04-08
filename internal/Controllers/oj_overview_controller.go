package controllers

import (
	"net/http"

	overviewsvc "patreonoj/internal/services/overview"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OjOverViewController struct {
	overviewService *overviewsvc.OverviewService
}

func NewOjOverViewController(db *gorm.DB) *OjOverViewController {
	return &OjOverViewController{
		overviewService: overviewsvc.NewOverviewService(db),
	}
}

// GetHomeText 获取OJ首页文本
func (oc *OjOverViewController) GetHomeText(c *gin.Context) {
	homeText, err := oc.overviewService.GetHomeText()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"home_text": homeText,
	})
}

// UpdateHomeText 更新OJ首页文本
func (oc *OjOverViewController) UpdateHomeText(c *gin.Context) {
	var input struct {
		HomeText string `json:"home_text"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	homeText, err := oc.overviewService.UpdateHomeText(input.HomeText)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":      200,
		"home_text": homeText,
	})
}

// GetAnnouncement 获取OJ公告
func (oc *OjOverViewController) GetAnnouncement(c *gin.Context) {
	announcement, err := oc.overviewService.GetAnnouncement()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"announcement": announcement,
	})
}

// UpdateAnnouncement 更新OJ公告
func (oc *OjOverViewController) UpdateAnnouncement(c *gin.Context) {
	var input struct {
		Announcement string `json:"announcement"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	announcement, err := oc.overviewService.UpdateAnnouncement(input.Announcement)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":         200,
		"announcement": announcement,
	})
}
