// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

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

// Cleans up a string by trimming \r and \n characters.
func CleanupString(s string) string {
	return strings.Trim(strings.Trim(s, "\n"), "\r")
}

// CleanupStrings cleans up a slice of strings by trimming \r and \n characters.
func CleanupStrings(s []string) []string {
	return Map(s, CleanupString)
}

// Formats an amount of base units as a string representing the amount in the given denomination.
// (i.e. An amount of 54321 with a decimals value of 3 results in the stirng "54.321")
func FormatAmount(amount *big.Int, decimals uint8) string {
	amountFloat := new(big.Float).SetInt(amount)
	divisor := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
	val := new(big.Float).Quo(amountFloat, divisor)
	return fmt.Sprintf("%.*f", decimals, val)
}

// Removes the leading 0x/0X part of a hexadecimal string representation
func TrimHexa(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}
