package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_moreThanOneVmSelected_success(t *testing.T) {
	*useSubnetEvm = true
	// *useSpaces = false
	// *useBlob = false
	// *useTimestamp = false
	*useCustom = false

	result := moreThanOneVmSelected()
	assert.False(t, result)
}

func Test_moreThanOneVmSelected_success_reverse(t *testing.T) {
	*useSubnetEvm = false
	// *useSpaces = false
	// *useBlob = false
	// *useTimestamp = false
	*useCustom = true

	result := moreThanOneVmSelected()
	assert.False(t, result)
}

func Test_moreThanOneVmSelected_success_none(t *testing.T) {
	*useSubnetEvm = false
	// *useSpaces = false
	// *useBlob = false
	// *useTimestamp = false
	*useCustom = false

	result := moreThanOneVmSelected()
	assert.False(t, result)
}

func Test_moreThanOneVmSelected_failure(t *testing.T) {
	*useSubnetEvm = true
	// *useSpaces = false
	// *useBlob = false
	// *useTimestamp = false
	*useCustom = true

	result := moreThanOneVmSelected()
	assert.True(t, result)
}
