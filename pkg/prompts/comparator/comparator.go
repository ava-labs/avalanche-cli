// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package comparator

import (
	"fmt"
)

// this package is needed to avoid circular dependencies on unit testing with prompter mock

const (
	LessThanEq = "Less Than Or Eq"
	MoreThanEq = "More Than Or Eq"
	MoreThan   = "More Than"
	NotEq      = "Not Eq"
)

type Comparator struct {
	Label string // Label that identifies reference value
	Type  string // Less Than Eq or More than Eq
	Value uint64 // Value to Compare To
}

func (comparator Comparator) Validate(val uint64) error {
	switch comparator.Type {
	case LessThanEq:
		if val > comparator.Value {
			return fmt.Errorf("the value must be smaller than or equal to %s (%d)", comparator.Label, comparator.Value)
		}
	case MoreThan:
		if val <= comparator.Value {
			return fmt.Errorf("the value must be bigger than %s (%d)", comparator.Label, comparator.Value)
		}
	case MoreThanEq:
		if val < comparator.Value {
			return fmt.Errorf("the value must be bigger than or equal to %s (%d)", comparator.Label, comparator.Value)
		}
	case NotEq:
		if val == comparator.Value {
			return fmt.Errorf("the value must be different than %s (%d)", comparator.Label, comparator.Value)
		}
	}
	return nil
}
