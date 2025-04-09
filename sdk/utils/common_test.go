// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAppendSlices tests AppendSlices
func TestAppendSlices(t *testing.T) {
	tests := []struct {
		name   string
		slices [][]interface{}
		want   []interface{}
	}{
		{
			name:   "AppendSlices with strings",
			slices: [][]interface{}{{"a", "b", "c"}, {"d", "e", "f"}, {"g", "h", "i"}},
			want:   []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		},
		{
			name:   "AppendSlices with ints",
			slices: [][]interface{}{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}},
			want:   []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			name:   "AppendSlices with empty slices",
			slices: [][]interface{}{{}, {}, {}},
			want:   []interface{}{},
		},
		{
			name:   "Append identical slices",
			slices: [][]interface{}{{"a", "b", "c"}, {"a", "b", "c"}},
			want:   []interface{}{"a", "b", "c", "a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppendSlices(tt.slices...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppendSlices() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Mock function for testing retries.
func mockFunction() (interface{}, error) {
	return nil, errors.New("error occurred")
}

// TestRetry tests Retry.
func TestRetry(t *testing.T) {
	success := "success"
	// Test with a function that always returns an error.
	result, err := Retry(mockFunction, 3, 100*time.Millisecond)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Test with a function that succeeds on the first attempt.
	fn := func() (interface{}, error) {
		return success, nil
	}
	result, err = Retry(fn, 3, 100*time.Millisecond)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != success {
		t.Errorf("Expected 'success' result, got %v", result)
	}

	// Test with a function that succeeds after multiple attempts.
	count := 0
	fn = func() (interface{}, error) {
		count++
		if count < 3 {
			return nil, errors.New("error occurred")
		}
		return success, nil
	}
	result, err = Retry(fn, 5, 100*time.Millisecond)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != success {
		t.Errorf("Expected 'success' result, got %v", result)
	}

	// Test with invalid retry interval.
	result, err = Retry(mockFunction, 3, 0)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

func TestRetryWithContextGen_Success(t *testing.T) {
	ctxGen := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), time.Second)
	}
	fn := func(_ context.Context) (int, error) {
		return 42, nil
	}
	result, err := RetryWithContextGen(ctxGen, fn, 3, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, 42, result)
}

func TestRetryWithContextGen_FailureAndRetry(t *testing.T) {
	ctxGen := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), time.Second)
	}
	attempts := 0
	fn := func(_ context.Context) (int, error) {
		attempts++
		if attempts < 2 {
			return 0, errors.New("temporary error")
		}
		return 42, nil
	}
	result, err := RetryWithContextGen(ctxGen, fn, 3, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, 42, result)
}

func TestRetryWithContextGen_ExceedMaxAttempts(t *testing.T) {
	ctxGen := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), time.Second)
	}
	fn := func(_ context.Context) (int, error) {
		return 0, errors.New("permanent error")
	}
	result, err := RetryWithContextGen(ctxGen, fn, 3, time.Millisecond)
	require.Error(t, err)
	require.Equal(t, 0, result)
}

func TestRetryWithContextGen_ContextCancellation(t *testing.T) {
	ctxGen := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), time.Millisecond)
	}
	fn := func(ctx context.Context) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		return 42, nil
	}
	result, err := RetryWithContextGen(ctxGen, fn, 3, time.Millisecond)
	require.Error(t, err)
	require.Equal(t, 0, result)
}

func TestWrapContext(t *testing.T) {
	// Test with a function that completes before the context timeout
	fn := func() (int, error) {
		return 42, nil
	}
	wrappedFn := WrapContext(fn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := wrappedFn(ctx)
	require.NoError(t, err)
	require.Equal(t, 42, result)
	// Test with a function that exceeds the context timeout
	fn = func() (int, error) {
		time.Sleep(2 * time.Second)
		return 42, nil
	}
	wrappedFn = WrapContext(fn)
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	result, err = wrappedFn(ctx)
	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
	require.Equal(t, 0, result)
}
