package admin

import "github.com/gin-gonic/gin"

func apiError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"code":  status,
		"msg":   message,
		"error": message,
	})
}

func apiOK(c *gin.Context, status int, message string, payload gin.H) {
	resp := gin.H{
		"code": status,
		"msg":  message,
	}
	for k, v := range payload {
		if _, exists := resp[k]; exists {
			continue
		}
		resp[k] = v
	}
	c.JSON(status, resp)
}
