package web

import (
	"net/http"
	"sync"
	"time"
)

// limiter 是进程内固定窗口限流器（单实例部署足够）。
// 说明：所有桶都是进程内状态，重启即清空；多副本部署需改用共享存储。
const (
	// 桶数量上限与固定 TTL：避免海量唯一 key（伪造邮箱/XFF）把内存撑爆（V-06）。
	limiterMaxKeys    = 50_000
	limiterBucketTTL  = 24 * time.Hour
	limiterPruneEvery = time.Minute
)

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
	l.pruneLocked(now)

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= window {
		l.buckets[key] = &bucket{start: now, count: 1}
		return true, 0
	}
	if b.count >= limit {
		return false, retrySeconds(now, b.start, window)
	}
	b.count++
	return true, 0
}

// blocked 只检查 key 是否已达上限，不增加计数。
// 用于“失败才计数”的账号级退避（M-01/M-02）：成功登录不应该消耗额度。
func (l *limiter) blocked(key string, limit int, window time.Duration) (bool, int) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= window {
		return false, 0
	}
	if b.count >= limit {
		return true, retrySeconds(now, b.start, window)
	}
	return false, 0
}

// hit 记录一次失败（窗口内累加）。
func (l *limiter) hit(key string, window time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= window {
		l.buckets[key] = &bucket{start: now, count: 1}
		return
	}
	b.count++
}

// clear 清空某个 key（例如登录成功后解除账号退避）。
func (l *limiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// pruneLocked 每分钟按固定 TTL 清理一次，并在 key 数超限时强制淘汰。
// 不能再用“当前调用的 window”清理，否则混合窗口下部分 key 永远不回收。
func (l *limiter) pruneLocked(now time.Time) {
	if now.Sub(l.lastSwap) >= limiterPruneEvery {
		for k, b := range l.buckets {
			if now.Sub(b.start) > limiterBucketTTL {
				delete(l.buckets, k)
			}
		}
		l.lastSwap = now
	}
	if len(l.buckets) <= limiterMaxKeys {
		return
	}
	// 超限时随机淘汰到 90%，保证内存有硬上限；被淘汰的 key 只是失去历史计数。
	target := limiterMaxKeys * 9 / 10
	for k := range l.buckets {
		if len(l.buckets) <= target {
			break
		}
		delete(l.buckets, k)
	}
}

func retrySeconds(now, start time.Time, window time.Duration) int {
	retry := int(window.Seconds() - now.Sub(start).Seconds())
	if retry < 1 {
		retry = 1
	}
	return retry
}

// limit 返回一个限流中间件。
func (s *Server) limit(limit int, window time.Duration, key func(*http.Request) string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			allow, retry := s.limiter.allow(key(r), limit, window)
			if !allow {
				w.Header().Set("Retry-After", itoa(retry))
				auditLog("rate_limited", "ip", clientIP(r, s.Cfg.TrustedProxies),
					"path", r.URL.Path, "retryAfter", retry)
				writeMessage(w, http.StatusTooManyRequests, "Too Many Attempts.")
				return
			}
			next(w, r)
		}
	}
}
