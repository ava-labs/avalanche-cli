package utils

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/sdk/constants"

	"github.com/stretchr/testify/require"
)

func TestUnique(t *testing.T) {
	// empty
	input := []int{}
	expected := []int{}
	result := Unique(input)
	require.Equal(t, expected, result)
	// ordered input
	input = []int{1, 2, 2, 3, 4, 4, 5}
	expected = []int{1, 2, 3, 4, 5}
	result = Unique(input)
	require.Equal(t, expected, result)
	// unordered input
	input = []int{5, 2, 1, 3, 4, 2, 4}
	expected = []int{5, 2, 1, 3, 4}
	result = Unique(input)
	require.Equal(t, expected, result)
}

func TestBelongs(t *testing.T) {
	// empty
	input := []string{}
	require.False(t, Belongs(input, "orange"))
	// non empty
	input = []string{"apple", "banana", "cherry"}
	require.True(t, Belongs(input, "banana"))
	require.False(t, Belongs(input, "orange"))
}

func TestUint32Sort(t *testing.T) {
	// empty
	input := []uint32{}
	expected := []uint32{}
	Uint32Sort(input)
	require.Equal(t, expected, input)
	// non empty
	input = []uint32{5, 3, 4, 1, 2}
	expected = []uint32{1, 2, 3, 4, 5}
	Uint32Sort(input)
	require.Equal(t, expected, input)
}

func TestGetAPIContext(t *testing.T) {
	ctx, cancel := GetAPIContext()
	deadline, hasDeadline := ctx.Deadline()
	timeout := time.Until(deadline)
	select {
	case <-ctx.Done():
		// Context should not be done immediately
		t.Fatal("context should not be done immediately")
	default:
		// Context is still active
	}
	require.Equal(t, true, hasDeadline)
	require.Equal(t, true, timeout-constants.APIRequestTimeout < time.Millisecond)
	cancel()
	select {
	case <-ctx.Done():
		// Context should be done immediately
	default:
		// Context is still active
		t.Fatal("context should be done immediately")
	}
}

func TestGetAPILargeContext(t *testing.T) {
	ctx, cancel := GetAPILargeContext()
	deadline, hasDeadline := ctx.Deadline()
	timeout := time.Until(deadline)
	select {
	case <-ctx.Done():
		// Context should not be done immediately
		t.Fatal("context should not be done immediately")
	default:
		// Context is still active
	}
	require.Equal(t, true, hasDeadline)
	require.Equal(t, true, timeout-constants.APIRequestLargeTimeout < time.Millisecond)
	cancel()
	select {
	case <-ctx.Done():
		// Context should be done immediately
	default:
		// Context is still active
		t.Fatal("context should be done immediately")
	}
}

func TestGetTimedContext(t *testing.T) {
	timeout := 50 * time.Millisecond
	ctx, cancel := GetTimedContext(timeout)
	defer cancel()
	select {
	case <-ctx.Done():
		// Context should not be done immediately
		t.Fatal("context should not be done immediately")
	default:
		// Context is still active
	}
	time.Sleep(timeout)
	time.Sleep(1 * time.Millisecond)
	select {
	case <-ctx.Done():
		// Context should be done after timeout
	default:
		t.Fatal("context should be done after timeout")
	}
}
