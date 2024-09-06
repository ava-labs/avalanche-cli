// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/exp/slices"
)

func Any[T any](input []T, f func(T) bool) bool {
	for _, e := range input {
		if f(e) {
			return true
		}
	}
	return false
}

func Find[T any](input []T, f func(T) bool) *T {
	for _, e := range input {
		if f(e) {
			return &e
		}
	}
	return nil
}

func Belongs[T comparable](input []T, elem T) bool {
	for _, e := range input {
		if e == elem {
			return true
		}
	}
	return false
}

func Filter[T any](input []T, f func(T) bool) []T {
	output := make([]T, 0, len(input))
	for _, e := range input {
		if f(e) {
			output = append(output, e)
		}
	}
	return output
}

func Map[T, U any](input []T, f func(T) U) []U {
	output := make([]U, 0, len(input))
	for _, e := range input {
		output = append(output, f(e))
	}
	return output
}

func MapWithError[T, U any](input []T, f func(T) (U, error)) ([]U, error) {
	output := make([]U, 0, len(input))
	for _, e := range input {
		o, err := f(e)
		if err != nil {
			return nil, err
		}
		output = append(output, o)
	}
	return output, nil
}

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

func CallWithTimeout[T any](
	name string,
	f func() (T, error),
	timeout time.Duration,
) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ret, err := WrapContext(f)(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("%s timeout of %d seconds", name, uint(timeout.Seconds()))
	}
	return ret, err
}

// RandomString generates a random string of the specified length.
func RandomString(length int) string {
	randG := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404
	chars := "abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[randG.Intn(len(chars))]
	}
	return string(result)
}

func SupportedAvagoArch() []string {
	return []string{string(types.ArchitectureTypeArm64), string(types.ArchitectureTypeX8664)}
}

func ArchSupported(arch string) bool {
	return slices.Contains(SupportedAvagoArch(), arch)
}
