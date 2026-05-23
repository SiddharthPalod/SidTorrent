package util

import (
	"sync"
	"time"
)

type RateLimiter struct {
	rate       int64
	capacity   int64
	tokens     int64
	lastUpdate time.Time
	mu         sync.Mutex
}

func NewRateLimiter(rate int64) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		capacity:   rate,
		tokens:     rate,
		lastUpdate: time.Now(),
	}
}
func (rl *RateLimiter) Wait(tokens int64) {
	if rl == nil || rl.rate <= 0 {
		return
	}
	for {
		rl.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(rl.lastUpdate).Seconds()
		rl.lastUpdate = now
		rl.tokens += int64(elapsed * float64(rl.rate))
		maxCap := rl.capacity
		if tokens > maxCap {
			maxCap = tokens
		}
		if rl.tokens > maxCap {
			rl.tokens = maxCap
		}
		if rl.tokens >= tokens {
			rl.tokens -= tokens
			rl.mu.Unlock()
			return
		}

		needed := tokens - rl.tokens
		sleepTime := time.Duration(float64(needed) / float64(rl.rate) * float64(time.Second))
		rl.mu.Unlock()
		time.Sleep(sleepTime)
	}
}
