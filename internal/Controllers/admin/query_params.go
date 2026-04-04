package admin

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func parsePositiveIntQuery(c *gin.Context, key string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s 必须是正整数", key)
	}
	return value, nil
}

func parsePageSizeQuery(c *gin.Context, pageKey, sizeKey string, defaultPage, defaultSize, maxSize int) (int, int, error) {
	page, err := parsePositiveIntQuery(c, pageKey, defaultPage)
	if err != nil {
		return 0, 0, err
	}

	size, err := parsePositiveIntQuery(c, sizeKey, defaultSize)
	if err != nil {
		return 0, 0, err
	}
	if size > maxSize {
		size = maxSize
	}
	return page, size, nil
}
