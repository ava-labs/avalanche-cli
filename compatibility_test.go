package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLatestCLISupportedDependencyVersion(t *testing.T) {
	testCases := []struct {
		name           string
		dependency     string
		expectedError  bool
		expectedResult string
	}{
		{
			name:           "valid dependency",
			dependency:     "github.com/example/package",
			expectedError:  false,
			expectedResult: "v1.0.0",
		},
		{
			name:           "empty dependency",
			dependency:     "",
			expectedError:  true,
			expectedResult: "",
		},
		{
			name:           "invalid dependency format",
			dependency:     "invalid/format",
			expectedError:  true,
			expectedResult: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetLatestCLISupportedDependencyVersion(tc.dependency)

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedResult, result)
			}
		})
	}
}
