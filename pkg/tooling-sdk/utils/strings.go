// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// containsIgnoreCase checks if the given string contains the specified substring, ignoring case.
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Cleans up a string by trimming \r and \n characters.
func CleanupString(s string) string {
	return strings.Trim(strings.Trim(s, "\n"), "\r")
}

// CleanupStrings cleans up a slice of strings by trimming \r and \n characters.
func CleanupStrings(s []string) []string {
	return Map(s, CleanupString)
}

// ExtractValueFromBytes extracts a value from a byte array using a regular expression.
func ExtractPlaceholderValue(pattern, text string) (string, error) {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) == 2 {
		return matches[1], nil
	} else {
		return "", fmt.Errorf("no match found")
	}
}

// AddSingleQuotes adds single quotes to each string in the given slice.
func AddSingleQuotes(s []string) []string {
	return Map(s, func(item string) string {
		if item == "" {
			return "''"
		}
		if !strings.HasPrefix(item, "'") {
			item = fmt.Sprintf("'%s", item)
		}
		if !strings.HasSuffix(item, "'") {
			item = fmt.Sprintf("%s'", item)
		}
		if !strings.HasPrefix(item, "'") && !strings.HasSuffix(item, "'") {
			item = fmt.Sprintf("'%s'", item)
		}
		return item
	})
}

// StringValue returns the value of a key in a map as a string.
func StringValue(data map[string]interface{}, key string) (string, error) {
	if value, ok := data[key]; ok {
		return fmt.Sprintf("%v", value), nil
	}
	return "", fmt.Errorf("key %s not found", key)
}
