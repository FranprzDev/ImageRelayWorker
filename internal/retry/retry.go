package retry

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

type PermanentError struct{ Err error }

func (e PermanentError) Error() string { return e.Err.Error() }
func (e PermanentError) Unwrap() error { return e.Err }

type RetryableError struct{ Err error }

func (e RetryableError) Error() string { return e.Err.Error() }
func (e RetryableError) Unwrap() error { return e.Err }

func IsPermanent(err error) bool {
	var target PermanentError
	return errors.As(err, &target)
}

func Do(ctx context.Context, attempts, baseMS int, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			last = err
			if IsPermanent(err) || attempt == attempts-1 {
				return err
			}
		}
		delay := time.Duration(baseMS) * time.Millisecond * time.Duration(1<<attempt)
		jitter := time.Duration(rand.Int63n(int64(delay/2 + 1)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay + jitter):
		}
	}
	return last
}
