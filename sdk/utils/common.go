// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"context"
	"fmt"
	"os"
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
	fn func(context.Context) (T, error),
	attempTimeout time.Duration,
	maxAttempts int,
	errMsg string,
) (T, error) {
	const defaultAttempTimeout = 2 * time.Second
	if attempTimeout == 0 {
		attempTimeout = defaultAttempTimeout
	}
	var (
		result T
		err    error
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), attempTimeout)
		defer cancel()
		result, err = fn(ctx)
		if err == nil {
			return result, nil
		}
		elapsed := time.Since(start)
		if elapsed < attempTimeout {
			time.Sleep(attempTimeout - elapsed)
		}
	}
	return result, fmt.Errorf(
		"%s: maximum retry attempts %d reached: last err = %w",
		errMsg,
		maxAttempts,
		err,
	)
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

func SDKUnitTestingEnabled() bool {
	return os.Getenv("SDK_UNIT_TESTING_ENABLED") != ""
}
