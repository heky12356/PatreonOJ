package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthThrottle bounds both Argon2 memory use and repeated requests per client.
// Login and registration must share the same handler instance.
func AuthThrottle(maxConcurrent, perMinute int) gin.HandlerFunc {
	if maxConcurrent < 1 {
		maxConcurrent = 2
	}
	if perMinute < 1 {
		perMinute = 20
	}
	slots := make(chan struct{}, maxConcurrent)
	type bucket struct {
		start time.Time
		count int
	}
	clients := make(map[string]bucket)
	var mu sync.Mutex
	return func(c *gin.Context) {
		now, ip := time.Now(), c.ClientIP()
		mu.Lock()
		_, exists := clients[ip]
		b := clients[ip]
		if now.Sub(b.start) >= time.Minute {
			b = bucket{start: now}
		}
		if _, exists := clients[ip]; !exists && len(clients) >= 4096 {
			for key, entry := range clients {
				if now.Sub(entry.start) >= time.Minute {
					delete(clients, key)
				}
			}
		}
		blocked := b.count >= perMinute || (!exists && len(clients) >= 4096)
		if !blocked {
			b.count++
			clients[ip] = b
		}
		mu.Unlock()
		if blocked {
			c.Header("Retry-After", strconv.Itoa(60))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "msg": "too many authentication requests; retry later"})
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			c.Next()
		default:
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "msg": "authentication is busy; retry later"})
		}
	}
}
