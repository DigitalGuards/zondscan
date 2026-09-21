package aiexplain

import (
	"errors"
	"time"
)

var (
	ErrGenerationInProgress = errors.New("AI explanation generation is already in progress")
	ErrDailyBudget          = errors.New("AI explanation daily provider budget reached")
	ErrBudgetCooldown       = errors.New("AI explanation provider budget is unavailable")
	ErrProviderCooldown     = errors.New("AI explanation provider call failed")
	ErrCacheCooldown        = errors.New("AI explanation cache write failed")
	ErrStorageUnavailable   = errors.New("AI explanation storage is unavailable")
)

type retryableError struct {
	kind       error
	cause      error
	retryAfter time.Duration
}

func (e *retryableError) Error() string {
	if e.cause == nil {
		return e.kind.Error()
	}
	return e.kind.Error() + ": " + e.cause.Error()
}

func (e *retryableError) Unwrap() []error {
	if e.cause == nil {
		return []error{e.kind}
	}
	return []error{e.kind, e.cause}
}

func (e *retryableError) RetryAfter() time.Duration {
	return e.retryAfter
}

// RetryAfter returns the retry delay carried by a typed explanation error.
func RetryAfter(err error) (time.Duration, bool) {
	var retryable interface {
		RetryAfter() time.Duration
	}
	if !errors.As(err, &retryable) {
		return 0, false
	}
	delay := retryable.RetryAfter()
	if delay < time.Second {
		delay = time.Second
	}
	return delay, true
}
