package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func graphJSONError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func graphParamInt(c *gin.Context, name, errMessage string) (int, bool) {
	value, err := strconv.Atoi(c.Param(name))
	if err != nil {
		graphJSONError(c, http.StatusBadRequest, errMessage)
		return 0, false
	}
	return value, true
}
