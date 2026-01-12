package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TokenBucket 令牌桶
type TokenBucket struct {
	capacity   int64         // 桶容量
	tokens     int64         // 当前令牌数
	rate       int64         // 每秒产生令牌数
	lastRefill time.Time     // 上次填充时间
	mu         sync.Mutex
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(capacity, rate int64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// Allow 尝试获取一个令牌
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += int64(elapsed * float64(tb.rate))
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// RateLimiterConfig 限流器配置
type RateLimiterConfig struct {
	GlobalQPS   int64 // 全局 QPS
	IPQPSLimit  int64 // 单 IP QPS
	UserQPSLimit int64 // 单用户 QPS
	BurstSize   int64 // 突发大小
}

// DefaultRateLimiterConfig 默认配置
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		GlobalQPS:    1000,
		IPQPSLimit:   50,
		UserQPSLimit: 100,
		BurstSize:    10,
	}
}

// RateLimiter 多级限流器
type RateLimiter struct {
	config       RateLimiterConfig
	globalBucket *TokenBucket
	ipBuckets    sync.Map // IP -> *TokenBucket
	userBuckets  sync.Map // UserID -> *TokenBucket
}

// NewRateLimiter 创建限流器
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		config:       config,
		globalBucket: NewTokenBucket(config.GlobalQPS+config.BurstSize, config.GlobalQPS),
	}
}

// getIPBucket 获取或创建 IP 限流桶
func (rl *RateLimiter) getIPBucket(ip string) *TokenBucket {
	if bucket, ok := rl.ipBuckets.Load(ip); ok {
		return bucket.(*TokenBucket)
	}
	bucket := NewTokenBucket(rl.config.IPQPSLimit+rl.config.BurstSize, rl.config.IPQPSLimit)
	actual, _ := rl.ipBuckets.LoadOrStore(ip, bucket)
	return actual.(*TokenBucket)
}

// getUserBucket 获取或创建用户限流桶
func (rl *RateLimiter) getUserBucket(userID string) *TokenBucket {
	if bucket, ok := rl.userBuckets.Load(userID); ok {
		return bucket.(*TokenBucket)
	}
	bucket := NewTokenBucket(rl.config.UserQPSLimit+rl.config.BurstSize, rl.config.UserQPSLimit)
	actual, _ := rl.userBuckets.LoadOrStore(userID, bucket)
	return actual.(*TokenBucket)
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(ip, userID string) bool {
	// 1. 检查全局限流
	if !rl.globalBucket.Allow() {
		return false
	}

	// 2. 检查 IP 限流
	if !rl.getIPBucket(ip).Allow() {
		return false
	}

	// 3. 检查用户限流（如果已登录）
	if userID != "" {
		if !rl.getUserBucket(userID).Allow() {
			return false
		}
	}

	return true
}

// Middleware Gin 中间件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if uid, ok := userID.(string); ok {
			userIDStr = uid
		}

		if !rl.Allow(ip, userIDStr) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Cleanup 清理过期的桶（定期调用）
func (rl *RateLimiter) Cleanup() {
	// 简单实现：清理超过1小时未使用的桶
	// 实际生产中可以用 LRU 或更复杂的策略
	rl.ipBuckets.Range(func(key, value interface{}) bool {
		bucket := value.(*TokenBucket)
		bucket.mu.Lock()
		if time.Since(bucket.lastRefill) > time.Hour {
			rl.ipBuckets.Delete(key)
		}
		bucket.mu.Unlock()
		return true
	})

	rl.userBuckets.Range(func(key, value interface{}) bool {
		bucket := value.(*TokenBucket)
		bucket.mu.Lock()
		if time.Since(bucket.lastRefill) > time.Hour {
			rl.userBuckets.Delete(key)
		}
		bucket.mu.Unlock()
		return true
	})
}
