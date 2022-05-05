package vm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_removePrecompile_success(t *testing.T) {
	precompile1 := "allow list"
	precompile2 := "minter"
	precompileList := []string{precompile1, precompile2}

	shortenedList, err := removePrecompile(precompileList, precompile1)
	assert.NoError(t, err)
	assert.Equal(t, shortenedList, []string{precompile2})
}

func Test_removePrecompile_success_reverse(t *testing.T) {
	precompile1 := "allow list"
	precompile2 := "minter"
	precompileList := []string{precompile1, precompile2}

	shortenedList, err := removePrecompile(precompileList, precompile2)
	assert.NoError(t, err)
	assert.Equal(t, shortenedList, []string{precompile1})
}

func Test_removePrecompile_failure(t *testing.T) {
	precompile1 := "allow list"
	precompile2 := "minter"
	precompileList := []string{precompile1}

	_, err := removePrecompile(precompileList, precompile2)
	assert.EqualError(t, err, "String not in array")
}
