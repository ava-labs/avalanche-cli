// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func SetupRealtimeCLIOutput(cmd *exec.Cmd, redirectStdout bool, redirectStderr bool) (*bytes.Buffer, *bytes.Buffer) {
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	if redirectStdout {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuffer)
	} else {
		cmd.Stdout = io.MultiWriter(&stdoutBuffer)
	}
	if redirectStderr {
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuffer)
	} else {
		cmd.Stderr = io.MultiWriter(&stderrBuffer)
	}
	return &stdoutBuffer, &stderrBuffer
}

// SplitKeyValueStringToMap splits a string with multiple key-value pairs separated by delimiter.
// Delimiter must be a single character
func SplitKeyValueStringToMap(str string, delimiter string) (map[string]string, error) {
	kvMap := make(map[string]string)
	if str == "" || len(delimiter) != 1 {
		return kvMap, nil
	}
	entries := SplitStringWithQuotes(str, rune(delimiter[0]))
	for _, e := range entries {
		parts := strings.Split(e, "=")
		if len(parts) >= 2 {
			kvMap[parts[0]] = strings.Trim(strings.Join(parts[1:], "="), "'")
		} else {
			kvMap[parts[0]] = strings.Trim(parts[0], "'")
		}
	}
	return kvMap, nil
}

// SplitString split string with a rune comma ignore quoted
func SplitStringWithQuotes(str string, r rune) []string {
	quoted := false
	return strings.FieldsFunc(str, func(r1 rune) bool {
		if r1 == '\'' {
			quoted = !quoted
		}
		return !quoted && r1 == r
	})
}

// Context for ANR network operations
func GetANRContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), constants.ANRRequestTimeout)
}

// Context for API requests
func GetAPIContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), constants.APIRequestTimeout)
}

func GetRealFilePath(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, _ := user.Current()
		path = strings.Replace(path, "~", usr.HomeDir, 1)
	}
	return path
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

// ConvertInterfaceToMap converts a given value to a map[string]interface{}.
func ConvertInterfaceToMap(value interface{}) (map[string]interface{}, error) {
	// Check if the underlying type is a map
	switch v := value.(type) {
	case map[string]interface{}:
		// If it's a map, return it
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", value)
	}
}

// SplitComaSeparatedString splits and trims a comma-separated string into a slice of strings.
func SplitComaSeparatedString(s string) []string {
	return Map(strings.Split(s, ","), strings.TrimSpace)
}

// SplitComaSeparatedInt splits a comma-separated string into a slice of integers.
func SplitComaSeparatedInt(s string) []int {
	return Map(SplitComaSeparatedString(s), func(item string) int {
		num, _ := strconv.Atoi(item)
		return num
	})
}

// IsUnsignedSlice returns true if all elements in the slice are unsigned integers.
func IsUnsignedSlice(n []int) bool {
	for _, v := range n {
		if v < 0 {
			return false
		}
	}
	return true
}

// TimedFunction is a function that executes the given function `f` within a specified timeout duration.
func TimedFunction(f func() (interface{}, error), name string, timeout time.Duration) (interface{}, error) {
	var (
		ret interface{}
		err error
	)
	ch := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	go func() {
		ret, err = f()
		close(ch)
	}()
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%s timeout of %d seconds", name, uint(timeout.Seconds()))
	case <-ch:
	}
	return ret, err
}

func SortUint32(arr []uint32) {
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
}

// Unique returns a new slice containing only the unique elements from the input slice.
func Unique(slice []string) []string {
	visited := make(map[string]bool)
	uniqueSlice := make([]string, 0)
	for _, element := range slice {
		if !visited[element] {
			// If the element is not visited, add it to the uniqueSlice
			uniqueSlice = append(uniqueSlice, element)
			visited[element] = true
		}
	}
	return uniqueSlice
}

// containsIgnoreCase checks if the given string contains the specified substring, ignoring case.
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
