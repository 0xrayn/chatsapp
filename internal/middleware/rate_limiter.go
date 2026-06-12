package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	tokens    float64
	maxTokens float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now

	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

type RateLimiter struct {
	buckets    map[string]*bucket
	mu         sync.RWMutex
	maxTokens  float64
	refillRate float64
	cleanupInterval time.Duration
}

func NewRateLimiter(maxRequests int, per time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets:         make(map[string]*bucket),
		maxTokens:       float64(maxRequests),
		refillRate:      float64(maxRequests) / per.Seconds(),
		cleanupInterval: 5 * time.Minute,
	}

	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	for range ticker.C {
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			b.mu.Lock()
			if time.Since(b.lastRefill) > 10*time.Minute {
				delete(rl.buckets, ip)
			}
			b.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) getBucket(ip string) *bucket {
	rl.mu.RLock()
	b, ok := rl.buckets[ip]
	rl.mu.RUnlock()

	if ok {
		return b
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if b, ok = rl.buckets[ip]; ok {
		return b
	}

	b = &bucket{
		tokens:     rl.maxTokens,
		maxTokens:  rl.maxTokens,
		refillRate: rl.refillRate,
		lastRefill: time.Now(),
	}
	rl.buckets[ip] = b
	return b
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		b := rl.getBucket(ip)

		if !b.allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"retry_after": "60s",
			})
			return
		}

		c.Next()
	}
}

// Stricter limiter for auth endpoints
func NewAuthRateLimiter() *RateLimiter {
	return NewRateLimiter(10, time.Minute) // 10 req/min per IP
}

// General API limiter
func NewAPIRateLimiter() *RateLimiter {
	return NewRateLimiter(120, time.Minute) // 120 req/min per IP
}
