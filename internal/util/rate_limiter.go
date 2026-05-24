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
	cond       *sync.Cond
	stopChan   chan struct{}
}

func NewRateLimiter(rate int64) *RateLimiter {
	rl := &RateLimiter{
		rate:       rate,
		capacity:   rate,
		tokens:     rate,
		lastUpdate: time.Now(),
		stopChan:   make(chan struct{}),
	}
	rl.cond = sync.NewCond(&rl.mu)

	// Start background refill goroutine to periodically replenish tokens every 100ms
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.mu.Lock()
				// Add 1/10th of the rate per 100ms
				rl.tokens += rl.rate / 10
				if rl.tokens > rl.capacity {
					rl.tokens = rl.capacity
				}
				rl.cond.Broadcast()
				rl.mu.Unlock()
			case <-rl.stopChan:
				return
			}
		}
	}()

	return rl
}

func (rl *RateLimiter) Wait(tokens int64) {
	if rl == nil || rl.rate <= 0 {
		return
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Ensure capacity is large enough
	if tokens > rl.capacity {
		rl.capacity = tokens
	}

	for rl.tokens < tokens {
		select {
		case <-rl.stopChan:
			return
		default:
		}
		rl.cond.Wait()
	}
	rl.tokens -= tokens
}

func (rl *RateLimiter) Close() {
	if rl == nil {
		return
	}
	rl.mu.Lock()
	select {
	case <-rl.stopChan:
		// already closed
	default:
		close(rl.stopChan)
	}
	rl.cond.Broadcast()
	rl.mu.Unlock()
}
