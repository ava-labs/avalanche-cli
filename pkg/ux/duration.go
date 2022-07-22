// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"strings"
	"time"
)

// FormatDuration returns a user friendly string for a duration
func FormatDuration(d time.Duration) string {
	var sb strings.Builder

	y := d / (24 * 365 * time.Hour)
	if y > 0 {
		d -= y * 24 * 365 * time.Hour
		sb.WriteString(fmt.Sprintf("%d years ", y))
	}
	dd := d / (24 * time.Hour)
	if dd > 0 {
		d -= dd * 24 * time.Hour
		sb.WriteString(fmt.Sprintf("%d days ", dd))
	}
	h := d / time.Hour
	if h > 0 {
		d -= h * time.Hour
		sb.WriteString(fmt.Sprintf("%d hours ", h))
	}
	m := d / time.Minute
	if m > 0 {
		d -= m * time.Minute
		sb.WriteString(fmt.Sprintf("%d minutes ", m))
	}
	s := d / time.Second
	if s > 0 {
		sb.WriteString(fmt.Sprintf("%d seconds ", s))
	}

	return sb.String()
}
