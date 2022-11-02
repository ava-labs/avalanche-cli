// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package flags

func EnsureMutuallyExclusive(flags []bool) bool {
	set := 0
	for _, f := range flags {
		if !f {
			continue
		}
		set++
		if set > 1 {
			return false
		}
	}

	return true
}
