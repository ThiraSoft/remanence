package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"remanence/internal/config"
	"remanence/internal/logger"
)

type ContextKey string

const RequestIDKey ContextKey = "requestID"

func GetRequestID(r *http.Request) string {
	if id, ok := r.Context().Value(RequestIDKey).(string); ok {
		return id
	}
	return "unknown"
}

func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// WithRequestID adds a unique ID to each request and logs request duration.
func WithRequestID(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)

		logger.Log.Debug("Request started", slog.String("id", requestID))
		start := time.Now()

		next(w, r.WithContext(ctx))

		logger.Log.Info("Request completed",
			slog.String("id", requestID),
			slog.Duration("duration", time.Since(start)))
	}
}

// SecurityHeaders adds security headers to every response.
func SecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next(w, r)
	}
}

// rateLimiter tracks per-IP request timestamps for rate limiting.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

var ipLimiter *rateLimiter

// InitRateLimiter sets up per-IP rate limiting.
func InitRateLimiter(limit int, window time.Duration) {
	ipLimiter = &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			ipLimiter.cleanup()
		}
	}()
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.requests[ip]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[ip] = valid
		return false
	}

	rl.requests[ip] = append(valid, now)
	return true
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for ip, timestamps := range rl.requests {
		valid := timestamps[:0]
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = valid
		}
	}
}

// RateLimitByIP enforces per-IP rate limiting on requests.
// Only trusts X-Forwarded-For when TRUST_PROXY is enabled,
// to prevent header spoofing from bypassing the rate limit.
func RateLimitByIP(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if config.TrustProxy {
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				// Take only the first IP (leftmost = original client)
				if i := len(forwarded); i > 0 {
					for j := 0; j < len(forwarded); j++ {
						if forwarded[j] == ',' {
							ip = forwarded[:j]
							break
						}
					}
					if !containsComma(forwarded) {
						ip = forwarded
					}
				}
			}
		}

		if !ipLimiter.allow(ip) {
			logger.Log.Warn("Rate limit exceeded", slog.String("ip", ip))
			http.Error(w, "Too many requests, please slow down", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func containsComma(s string) bool {
	for _, c := range s {
		if c == ',' {
			return true
		}
	}
	return false
}

// Chain applies multiple middlewares to a http.HandlerFunc.
func Chain(handler http.HandlerFunc, middlewares ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
