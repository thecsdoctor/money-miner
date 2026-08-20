// Package ratelimit provides small in-process token-bucket limiters keyed by
// IP or user id (dossier 04/07: public POSTs 10-30/min/IP, authed 300/min/user).
package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter is a keyed set of token buckets.
type Limiter struct {
	mu   sync.Mutex
	m    map[string]*rate.Limiter
	r    rate.Limit
	b    int
	last time.Time
}

// New creates a limiter: r events per minute per key, burst b.
func New(perMinute float64, burst int) *Limiter {
	return &Limiter{m: map[string]*rate.Limiter{}, r: rate.Limit(perMinute / 60), b: burst, last: time.Now()}
}

// Allow reports whether key may proceed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	if len(l.m) > 10000 || time.Since(l.last) > 10*time.Minute {
		// cheap eviction: drop the whole map periodically; limits re-warm
		l.m = map[string]*rate.Limiter{}
		l.last = time.Now()
	}
	lim, ok := l.m[key]
	if !ok {
		lim = rate.NewLimiter(l.r, l.b)
		l.m[key] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}

// ClientIP extracts the client IP; the nginx edge sets X-Forwarded-For and
// real client IP arrives via PROXY protocol at the edge. Behind the edge,
// RemoteAddr is loopback, so X-Forwarded-For is authoritative here.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Middleware rejects requests over the key's budget with the standard error
// envelope. keyFn chooses the key (IP, user sub, ...).
func (l *Limiter) Middleware(keyFn func(*http.Request) string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(keyFn(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many requests — slow down"}}`))
			return
		}
		next(w, r)
	}
}
