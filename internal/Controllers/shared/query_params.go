package shared

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func ParsePositiveIntQuery(c *gin.Context, key string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s 参数错误", key)
	}
	return value, nil
}

func ParsePageSizeQuery(c *gin.Context, pageKey, sizeKey string, defaultPage, defaultSize, maxSize int) (int, int, error) {
	page, err := ParsePositiveIntQuery(c, pageKey, defaultPage)
	if err != nil {
		return 0, 0, err
	}

	size, err := ParsePositiveIntQuery(c, sizeKey, defaultSize)
	if err != nil {
		return 0, 0, err
	}
	if size > maxSize {
		size = maxSize
	}
	return page, size, nil
}
