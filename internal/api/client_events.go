package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/middleware"
	"azule.info/calorize/internal/telemetry"
)

var (
	urlParamRegex = regexp.MustCompile(`(\?|#)[^\s"'>]*`)
	tokenRegex    = regexp.MustCompile(`(?i)(bearer\s+[a-z0-9_\-\.~+/]+=*|v2\.(?:local|public)\.[a-z0-9_\-]+)`)
)

type ClientEvent struct {
	Kind     string `json:"kind"`
	Message  string `json:"message"`
	Location string `json:"location,omitempty"`
	Route    string `json:"route"`
	Build    string `json:"build,omitempty"`
	TS       int64  `json:"ts,omitempty"`
}

type ClientEventsBatch struct {
	Events []ClientEvent `json:"events"`
}

type tokenBucket struct {
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newTokenBucket(capacity float64, perMinute float64) *tokenBucket {
	return &tokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: perMinute / 60.0,
		lastRefill: time.Now(),
	}
}

func (tb *tokenBucket) allow(now time.Time) bool {
	elapsed := now.Sub(tb.lastRefill).Seconds()
	if elapsed > 0 {
		tb.tokens += elapsed * tb.refillRate
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

type clientEventLimiter struct {
	mu          sync.Mutex
	userGlobal  *tokenBucket
	userBuckets map[string]*tokenBucket
	anonGlobal  *tokenBucket
	anonBuckets map[string]*tokenBucket
	lastPrune   time.Time
}

func newClientEventLimiter() *clientEventLimiter {
	return &clientEventLimiter{
		userGlobal:  newTokenBucket(600, 600),
		userBuckets: make(map[string]*tokenBucket),
		anonGlobal:  newTokenBucket(300, 300),
		anonBuckets: make(map[string]*tokenBucket),
	}
}

func (l *clientEventLimiter) prune(now time.Time) {
	if now.Sub(l.lastPrune) < time.Minute {
		return
	}
	l.lastPrune = now

	if len(l.anonBuckets) > 1000 {
		for k, b := range l.anonBuckets {
			if now.Sub(b.lastRefill) > 5*time.Minute {
				delete(l.anonBuckets, k)
			}
		}
		if len(l.anonBuckets) > 2000 {
			l.anonBuckets = make(map[string]*tokenBucket)
		}
	}
	if len(l.userBuckets) > 1000 {
		for k, b := range l.userBuckets {
			if now.Sub(b.lastRefill) > 5*time.Minute {
				delete(l.userBuckets, k)
			}
		}
		if len(l.userBuckets) > 2000 {
			l.userBuckets = make(map[string]*tokenBucket)
		}
	}
}

func (l *clientEventLimiter) Allow(userID string, ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.prune(now)

	if userID != "" {
		bucket, exists := l.userBuckets[userID]
		if !exists {
			bucket = newTokenBucket(30, 30)
			l.userBuckets[userID] = bucket
		}
		if !bucket.allow(now) {
			return false
		}
		return l.userGlobal.allow(now)
	}

	bucket, exists := l.anonBuckets[ip]
	if !exists {
		bucket = newTokenBucket(10, 10)
		l.anonBuckets[ip] = bucket
	}
	if !bucket.allow(now) {
		return false
	}
	return l.anonGlobal.allow(now)
}

var limiter = newClientEventLimiter()

// ResetClientEventsLimiter resets in-memory rate limiting state (used for tests).
func ResetClientEventsLimiter() {
	limiter = newClientEventLimiter()
}

// SanitizeString removes query strings, hash fragments, auth tokens, and truncates to 500 characters.
func SanitizeString(s string) string {
	cleaned := urlParamRegex.ReplaceAllString(s, "")
	cleaned = tokenRegex.ReplaceAllString(cleaned, "[REDACTED]")
	runes := []rune(cleaned)
	if len(runes) > 500 {
		return string(runes[:500])
	}
	return cleaned
}

func extractUserID(ctx context.Context) string {
	if uid, ok := auth.UserIDFromContext(ctx); ok {
		return uid.String()
	}
	return ""
}

// getClientIP returns the address used as the anonymous rate-limit key.
//
// It takes the right-most X-Forwarded-For entry: that is the one appended by
// the trusted host proxy in front of nginx, which passes the header through
// unchanged. Entries to its left are supplied by the client and can be forged,
// so using the left-most entry would let a caller pick its own bucket.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
			return ip
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// RegisterClientEventsPaths registers the POST /client-events endpoint on the mux.
func RegisterClientEventsPaths(mux *http.ServeMux) {
	mux.HandleFunc("POST /client-events", handleClientEvents)
}

func handleClientEvents(w http.ResponseWriter, r *http.Request) {
	authUserID := extractUserID(r.Context())
	clientIP := getClientIP(r)

	if !limiter.Allow(authUserID, clientIP) {
		telemetry.ClientEventsDroppedTotal.WithLabelValues("rate_limit").Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "rate limit exceeded",
			"code":  "RATE_LIMITED",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16384) // 16 KiB cap
	var batch ClientEventsBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			telemetry.ClientEventsDroppedTotal.WithLabelValues("payload_too_large").Inc()
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(batch.Events) > 20 {
		telemetry.ClientEventsDroppedTotal.WithLabelValues("batch_too_large").Inc()
		http.Error(w, "batch size exceeds limit of 20 events", http.StatusBadRequest)
		return
	}

	reqID := middleware.GetRequestID(r.Context())
	userIDStr := authUserID
	if userIDStr == "" {
		userIDStr = "anonymous"
	}

	for _, ev := range batch.Events {
		slog.WarnContext(r.Context(), "client.error",
			"kind", SanitizeString(ev.Kind),
			"message", SanitizeString(ev.Message),
			"location", SanitizeString(ev.Location),
			"route", SanitizeString(ev.Route),
			"build", SanitizeString(ev.Build),
			"request_id", reqID,
			"user_id", userIDStr,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}
