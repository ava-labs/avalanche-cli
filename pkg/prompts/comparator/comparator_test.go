// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package comparator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComparator_Validate(t *testing.T) {
	tests := []struct {
		name       string
		comparator Comparator
		value      uint64
		wantErr    bool
	}{
		// LessThanEq tests
		{
			name: "LessThanEq - value equal to limit",
			comparator: Comparator{
				Label: "max value",
				Type:  LessThanEq,
				Value: 100,
			},
			value:   100,
			wantErr: false,
		},
		{
			name: "LessThanEq - value less than limit",
			comparator: Comparator{
				Label: "max value",
				Type:  LessThanEq,
				Value: 100,
			},
			value:   50,
			wantErr: false,
		},
		{
			name: "LessThanEq - value greater than limit",
			comparator: Comparator{
				Label: "max value",
				Type:  LessThanEq,
				Value: 100,
			},
			value:   150,
			wantErr: true,
		},
		{
			name: "LessThanEq - zero value within limit",
			comparator: Comparator{
				Label: "max value",
				Type:  LessThanEq,
				Value: 100,
			},
			value:   0,
			wantErr: false,
		},

		// MoreThan tests
		{
			name: "MoreThan - value greater than limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThan,
				Value: 50,
			},
			value:   100,
			wantErr: false,
		},
		{
			name: "MoreThan - value equal to limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThan,
				Value: 50,
			},
			value:   50,
			wantErr: true,
		},
		{
			name: "MoreThan - value less than limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThan,
				Value: 50,
			},
			value:   25,
			wantErr: true,
		},
		{
			name: "MoreThan - zero value less than limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThan,
				Value: 50,
			},
			value:   0,
			wantErr: true,
		},

		// MoreThanEq tests
		{
			name: "MoreThanEq - value equal to limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThanEq,
				Value: 50,
			},
			value:   50,
			wantErr: false,
		},
		{
			name: "MoreThanEq - value greater than limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThanEq,
				Value: 50,
			},
			value:   100,
			wantErr: false,
		},
		{
			name: "MoreThanEq - value less than limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThanEq,
				Value: 50,
			},
			value:   25,
			wantErr: true,
		},
		{
			name: "MoreThanEq - zero value less than limit",
			comparator: Comparator{
				Label: "min value",
				Type:  MoreThanEq,
				Value: 50,
			},
			value:   0,
			wantErr: true,
		},

		// NotEq tests
		{
			name: "NotEq - value different from reference",
			comparator: Comparator{
				Label: "forbidden value",
				Type:  NotEq,
				Value: 100,
			},
			value:   50,
			wantErr: false,
		},
		{
			name: "NotEq - value equal to reference",
			comparator: Comparator{
				Label: "forbidden value",
				Type:  NotEq,
				Value: 100,
			},
			value:   100,
			wantErr: true,
		},
		{
			name: "NotEq - zero value different from reference",
			comparator: Comparator{
				Label: "forbidden value",
				Type:  NotEq,
				Value: 100,
			},
			value:   0,
			wantErr: false,
		},
		{
			name: "NotEq - zero value equal to zero reference",
			comparator: Comparator{
				Label: "forbidden value",
				Type:  NotEq,
				Value: 0,
			},
			value:   0,
			wantErr: true,
		},

		// Edge cases
		{
			name: "Unknown comparator type - should pass through",
			comparator: Comparator{
				Label: "test value",
				Type:  "UnknownType",
				Value: 100,
			},
			value:   50,
			wantErr: false,
		},
		{
			name: "Max uint64 value - LessThanEq",
			comparator: Comparator{
				Label: "max uint64",
				Type:  LessThanEq,
				Value: ^uint64(0), // max uint64
			},
			value:   ^uint64(0), // max uint64
			wantErr: false,
		},
		{
			name: "Max uint64 value - exceeds limit",
			comparator: Comparator{
				Label: "max value",
				Type:  LessThanEq,
				Value: ^uint64(0) - 1, // max uint64 - 1
			},
			value:   ^uint64(0), // max uint64
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.comparator.Validate(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				// Verify error message contains the label and value
				require.Contains(t, err.Error(), tt.comparator.Label)
				require.Contains(t, err.Error(), "must be")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestComparator_ValidateErrorMessages(t *testing.T) {
	tests := []struct {
		name               string
		comparator         Comparator
		value              uint64
		expectedErrMessage string
	}{
		{
			name: "LessThanEq error message",
			comparator: Comparator{
				Label: "max connections",
				Type:  LessThanEq,
				Value: 100,
			},
			value:              150,
			expectedErrMessage: "the value must be smaller than or equal to max connections (100)",
		},
		{
			name: "MoreThan error message",
			comparator: Comparator{
				Label: "minimum stake",
				Type:  MoreThan,
				Value: 2000,
			},
			value:              2000,
			expectedErrMessage: "the value must be bigger than minimum stake (2000)",
		},
		{
			name: "MoreThanEq error message",
			comparator: Comparator{
				Label: "required balance",
				Type:  MoreThanEq,
				Value: 1000,
			},
			value:              500,
			expectedErrMessage: "the value must be bigger than or equal to required balance (1000)",
		},
		{
			name: "NotEq error message",
			comparator: Comparator{
				Label: "reserved port",
				Type:  NotEq,
				Value: 80,
			},
			value:              80,
			expectedErrMessage: "the value must be different than reserved port (80)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.comparator.Validate(tt.value)
			require.Error(t, err)
			require.Equal(t, tt.expectedErrMessage, err.Error())
		})
	}
}
