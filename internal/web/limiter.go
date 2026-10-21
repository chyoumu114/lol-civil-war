package web

import (
	"net/http"
	"sync"
	"time"
)

// limiter 是进程内固定窗口限流器（单实例部署足够）。
type limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	lastSwap time.Time
}

type bucket struct {
	start time.Time
	count int
}

func newLimiter() *limiter {
	return &limiter{buckets: make(map[string]*bucket), lastSwap: time.Now()}
}

// allow 判断 key 在窗口内是否未超限；超限时返回需要等待的秒数。
func (l *limiter) allow(key string, limit int, window time.Duration) (bool, int) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// 顺带清理长时间不用的 key，避免内存无限增长。
	if now.Sub(l.lastSwap) > 10*time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.start) > window {
				delete(l.buckets, k)
			}
		}
		l.lastSwap = now
	}

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= window {
		l.buckets[key] = &bucket{start: now, count: 1}
		return true, 0
	}
	if b.count >= limit {
		retry := int(window.Seconds() - now.Sub(b.start).Seconds())
		if retry < 1 {
			retry = 1
		}
		return false, retry
	}
	b.count++
	return true, 0
}

// limit 返回一个限流中间件。
func (s *Server) limit(limit int, window time.Duration, key func(*http.Request) string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			allow, retry := s.limiter.allow(key(r), limit, window)
			if !allow {
				w.Header().Set("Retry-After", itoa(retry))
				writeMessage(w, http.StatusTooManyRequests, "Too Many Attempts.")
				return
			}
			next(w, r)
		}
	}
}
