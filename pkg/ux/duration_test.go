// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDurationFormat(t *testing.T) {
	assert := assert.New(t)

	type test struct {
		d        time.Duration
		expected string
	}

	tests := []test{
		{
			d:        972 * 24 * time.Hour,
			expected: "2 years 242 days ",
		},
		{
			d:        364 * 24 * time.Hour,
			expected: "364 days ",
		},
		{
			d:        42 * 24 * time.Hour,
			expected: "42 days ",
		},
		{
			d:        1 * 24 * time.Hour,
			expected: "1 days ",
		},
		{
			d:        23 * time.Hour,
			expected: "23 hours ",
		},
		{
			d:        42*time.Hour + 42*time.Minute,
			expected: "1 days 18 hours 42 minutes ",
		},
		{
			d:        42 * time.Minute,
			expected: "42 minutes ",
		},
		{
			d:        42*time.Minute + 42*time.Second,
			expected: "42 minutes 42 seconds ",
		},
		{
			d:        42 * time.Second,
			expected: "42 seconds ",
		},
		{
			d:        342*24*time.Hour + 42*time.Second,
			expected: "342 days 42 seconds ",
		},
		{
			d:        342*24*time.Hour + 1*time.Hour + 42*time.Second,
			expected: "342 days 1 hours 42 seconds ",
		},
		{
			d:        342*24*time.Hour + 42*time.Minute + 42*time.Second,
			expected: "342 days 42 minutes 42 seconds ",
		},
		{
			d:        342*24*time.Hour + 42*time.Minute,
			expected: "342 days 42 minutes ",
		},
	}

	for _, t := range tests {
		s := FormatDuration(t.d)
		assert.Equal(t.expected, s)
	}
}
