package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:generate mockgen -source=rate_limit.go -destination=../../../gen/mocks/api_gateway/middleware/rate_limiter_mock.go -package=middlewaremock

type RateLimiter interface {
	Allow(context.Context, string, int, time.Duration) (bool, error)
}
type RedisRateLimiter struct{ rdb *redis.Client }

func NewRedisRateLimiter(rdb *redis.Client) RateLimiter { return &RedisRateLimiter{rdb: rdb} }
func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	pipe := r.rdb.TxPipeline()
	n := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return n.Val() <= int64(limit), nil
}
func RateLimit(l RateLimiter, limit int, window time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "rate:" + clientKey(r)
		if p, ok := PrincipalFromContext(r.Context()); ok {
			key = fmt.Sprintf("rate:user:%s", p.UserID)
		}
		allowed, err := l.Allow(r.Context(), key, limit, window)
		if err != nil {
			http.Error(w, `{"code":"INTERNAL","message":"rate limiter unavailable"}`, 500)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
			http.Error(w, `{"code":"RATE_LIMITED","message":"rate limit exceeded"}`, 429)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func clientKey(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return v
	}
	return r.RemoteAddr
}
