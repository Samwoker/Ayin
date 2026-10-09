package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var tokenBucketLua = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local data = redis.call("HMGET", key, "tokens", "last_updated")
local tokens = tonumber(data[1])
local last_updated = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    last_updated = now
else
    local delta = math.max(0, now - last_updated)
    tokens = math.min(capacity, tokens + delta * rate)
    last_updated = now
end

if tokens < requested then
    redis.call("HMSET", key, "tokens", tokens, "last_updated", last_updated)
    redis.call("EXPIRE", key, math.max(60, math.ceil(capacity / rate)))
    return {0, math.floor(tokens)}
else
    tokens = tokens - requested
    redis.call("HMSET", key, "tokens", tokens, "last_updated", last_updated)
    redis.call("EXPIRE", key, math.max(60, math.ceil(capacity / rate)))
    return {1, math.floor(tokens)}
end
`)

// TierLimits defines token-bucket capacity and refill rates for each plan.
type TierLimits struct {
	Capacity   int     // Max bucket burst capacity
	RefillRate float64 // Tokens per second
	LimitRPM   int     // Requests per minute
}

var TierPolicies = map[string]TierLimits{
	"free": {
		Capacity:   20,
		RefillRate: 10.0 / 60.0,
		LimitRPM:   10,
	},
	"pro": {
		Capacity:   100,
		RefillRate: 60.0 / 60.0,
		LimitRPM:   60,
	},
	"enterprise": {
		Capacity:   500,
		RefillRate: 300.0 / 60.0,
		LimitRPM:   300,
	},
}

// RateLimiter manages request throttling per tenant or API key using a token-bucket algorithm.
type RateLimiter struct {
	redisClient *redis.Client
	limit       int           // Default max capacity (tokens)
	refillRate  float64       // Default tokens per second
	window      time.Duration // Duration to completely refill
	mu          sync.Mutex
	localCounts map[string]*localBucket
}

type localBucket struct {
	tokens      float64
	lastUpdated time.Time
}

// NewRateLimiter creates a RateLimiter instance with optional Redis backing.
func NewRateLimiter(redisClient *redis.Client, limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 120
	}
	if window <= 0 {
		window = time.Minute
	}
	refillRate := float64(limit) / window.Seconds()

	return &RateLimiter{
		redisClient: redisClient,
		limit:       limit,
		refillRate:  refillRate,
		window:      window,
		localCounts: make(map[string]*localBucket),
	}
}

func (rl *RateLimiter) resolveTierPolicy(tier string) TierLimits {
	if tier != "" {
		if p, ok := TierPolicies[tier]; ok {
			return p
		}
	}
	return TierLimits{
		Capacity:   rl.limit,
		RefillRate: rl.refillRate,
		LimitRPM:   rl.limit,
	}
}

// RateLimitMiddleware throttles incoming requests per organization and per API key according to plan tiers.
func (rl *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, hasTenant := GetTenant(r.Context())

		tier := "custom"
		if hasTenant && tc.PlanTier != "" {
			tier = tc.PlanTier
		} else if !hasTenant {
			tier = "unauthenticated"
		}

		policy := rl.resolveTierPolicy(tier)

		var keysToCheck []string
		if hasTenant {
			// Per-organization bucket
			keysToCheck = append(keysToCheck, fmt.Sprintf("org:%x", tc.OrgID.Bytes))

			// Per-key bucket if authenticated via API key
			if tc.IsAPIKey {
				keysToCheck = append(keysToCheck, fmt.Sprintf("key:%x", tc.APIKeyID.Bytes))
			}
		} else {
			// Unauthenticated request fallback: IP address bucket
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = strings.TrimSpace(strings.Split(forwarded, ",")[0])
			}
			keysToCheck = append(keysToCheck, fmt.Sprintf("ip:%s", ip))
		}

		minRemaining := policy.Capacity
		for _, key := range keysToCheck {
			allowed, remaining, err := rl.allow(r.Context(), key, policy.Capacity, policy.RefillRate)
			if err != nil {
				// Fail open on rate limiter storage errors
				continue
			}
			if remaining < minRemaining {
				minRemaining = remaining
			}
			if !allowed {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", policy.LimitRPM))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Tier", tier)
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":       fmt.Sprintf("Rate limit exceeded for tier '%s' (%d req/min). Upgrade to Pro for higher capacity.", tier, policy.LimitRPM),
					"tier":        tier,
					"limit_rpm":   policy.LimitRPM,
					"upgrade_url": "/billing",
				})
				return
			}
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", policy.LimitRPM))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", minRemaining))
		w.Header().Set("X-RateLimit-Tier", tier)

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ctx context.Context, key string, capacity int, refillRate float64) (bool, int, error) {
	if capacity <= 0 {
		capacity = rl.limit
	}
	if refillRate <= 0 {
		refillRate = rl.refillRate
	}

	if rl.redisClient != nil {
		redisKey := fmt.Sprintf("ratelimit:%s", key)
		nowUnix := time.Now().Unix()
		res, err := tokenBucketLua.Run(ctx, rl.redisClient, []string{redisKey}, capacity, refillRate, nowUnix, 1).Slice()
		if err == nil && len(res) == 2 {
			allowedCode, ok1 := res[0].(int64)
			remainingTokens, ok2 := res[1].(int64)
			if ok1 && ok2 {
				return allowedCode == 1, int(remainingTokens), nil
			}
		}
	}

	// In-memory fallback
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.localCounts[key]
	if !exists {
		rl.localCounts[key] = &localBucket{
			tokens:      float64(capacity - 1),
			lastUpdated: now,
		}
		return true, capacity - 1, nil
	}

	// Refill tokens according to elapsed duration
	elapsed := now.Sub(b.lastUpdated).Seconds()
	b.tokens = math.Min(float64(capacity), b.tokens+(elapsed*refillRate))
	b.lastUpdated = now

	if b.tokens < 1.0 {
		return false, 0, nil
	}

	b.tokens -= 1.0
	return true, int(b.tokens), nil
}
