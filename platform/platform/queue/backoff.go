package queue

import (
	"math/rand"
	"time"
)

type BackoffPolicy struct {
	Max         time.Duration
	MaxExponent int
	JitterDiv   int64
}

func NewExponentialBackoffPolicy(max time.Duration) BackoffPolicy {
	return BackoffPolicy{
		Max:         max,
		MaxExponent: 12,
		JitterDiv:   5,
	}
}

func (p BackoffPolicy) Next(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	maxExponent := p.MaxExponent
	if maxExponent <= 0 {
		maxExponent = 12
	}

	base := time.Second * time.Duration(1<<minInt(attempt-1, maxExponent))

	maxBackoff := p.Max
	if maxBackoff <= 0 {
		maxBackoff = 6 * time.Hour
	}
	if base > maxBackoff {
		base = maxBackoff
	}

	jitterDiv := p.JitterDiv
	if jitterDiv <= 0 {
		jitterDiv = 5
	}
	jitter := time.Duration(rand.Int63n(int64(base / time.Duration(jitterDiv))))
	wait := base + jitter
	if wait > maxBackoff {
		return maxBackoff
	}
	return wait
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
