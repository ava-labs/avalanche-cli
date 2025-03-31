// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// AppendSlices appends multiple slices into a single slice.
func AppendSlices[T any](slices ...[]T) []T {
	totalLength := 0
	for _, slice := range slices {
		totalLength += len(slice)
	}
	result := make([]T, 0, totalLength)
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}

// Retry retries the given function until it succeeds or the maximum number of attempts is reached.
func Retry[T any](
	fn func() (T, error),
	maxAttempts int,
	retryInterval time.Duration,
) (T, error) {
	const defaultRetryInterval = 2 * time.Second
	if retryInterval == 0 {
		retryInterval = defaultRetryInterval
	}
	var (
		result T
		cumErr error
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var err error
		result, err = fn()
		if err == nil {
			return result, nil
		}
		cumErr = errors.Join(cumErr, err)
		time.Sleep(retryInterval)
	}
	return result, fmt.Errorf(
		"maximum retry attempts %d reached: cumulated err = %w",
		maxAttempts,
		cumErr,
	)
}

// RetryWithContext retries the given function until it succeeds or the maximum number of attempts is reached.
// For each retry, it generates a fresh context to be used on the call
func RetryWithContextGen[T any](
	ctxGen func() (context.Context, context.CancelFunc),
	fn func(context.Context) (T, error),
	maxAttempts int,
	retryInterval time.Duration,
) (T, error) {
	newfn := func() (T, error) {
		ctx, cancel := ctxGen()
		defer cancel()
		return fn(ctx)
	}
	return Retry(newfn, maxAttempts, retryInterval)
}

// WrapContext adds a context based timeout to a given function
func WrapContext[T any](
	f func() (T, error),
) func(context.Context) (T, error) {
	return func(ctx context.Context) (T, error) {
		var (
			ret T
			err error
		)
		ch := make(chan struct{})
		go func() {
			ret, err = f()
			close(ch)
		}()
		select {
		case <-ctx.Done():
			return ret, ctx.Err()
		case <-ch:
		}
		return ret, err
	}
}
