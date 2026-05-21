package ratelimiter

import (
	"context"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiter *rate.Limiter
}

func NewRateLimiter(r, b int) *RateLimiter {
	return &RateLimiter{limiter: rate.NewLimiter(rate.Limit(r), b)}
}

// имплементация интерфейса для использования в resty
func (r *RateLimiter) Allow() bool {
	//используем Wait вместо Allow чтобы запросы выстраивались в очередь, а не падали
	if err := r.limiter.Wait(context.Background()); err != nil {
		return false
	}
	return true
}
