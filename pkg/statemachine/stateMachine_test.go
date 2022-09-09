// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statemachine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_StateMachineNoStates(t *testing.T) {
	assert := assert.New(t)

	machine, err := NewStateMachine([]string{})
	assert.Error(err)
	assert.Equal(machine, (*StateMachine)(nil))
}

func Test_StateMachineStop(t *testing.T) {
	assert := assert.New(t)

	machine, err := NewStateMachine([]string{"state"})
	assert.NoError(err)
	assert.Equal(machine.CurrentState(), "state")
	assert.Equal(machine.Running(), true)
	machine.Stop()
	assert.Equal(machine.CurrentState(), notRunningState)
	assert.Equal(machine.Running(), false)
}

func Test_StateMachineOneState(t *testing.T) {
	assert := assert.New(t)

	machine, err := NewStateMachine([]string{"state"})
	assert.NoError(err)
	assert.Equal(machine.CurrentState(), "state")
	assert.Equal(machine.Running(), true)
	machine.NextState(Forward)
	assert.Equal(machine.CurrentState(), notRunningState)
	assert.Equal(machine.Running(), false)
}

func Test_StateMachineNStates(t *testing.T) {
	assert := assert.New(t)

	states := []string{"one", "two", "...", "N"}
	machine, err := NewStateMachine(states)
	assert.NoError(err)
	for _, state := range states {
		assert.Equal(machine.CurrentState(), state)
		assert.Equal(machine.Running(), true)
		machine.NextState(Forward)
	}
	assert.Equal(machine.CurrentState(), notRunningState)
	assert.Equal(machine.Running(), false)
}

func Test_StateMachineBackward(t *testing.T) {
	assert := assert.New(t)

	states := []string{"one", "two", "...", "N"}
	machine, err := NewStateMachine(states)
	assert.NoError(err)
	machine.NextState(Backward)
	assert.Equal(machine.CurrentState(), states[0])
	assert.Equal(machine.Running(), true)
	for i := 0; i < len(states)-1; i++ {
		assert.Equal(machine.CurrentState(), states[i])
		assert.Equal(machine.Running(), true)
		machine.NextState(Forward)
		assert.Equal(machine.CurrentState(), states[i+1])
		assert.Equal(machine.Running(), true)
		machine.NextState(Backward)
		assert.Equal(machine.CurrentState(), states[i])
		assert.Equal(machine.Running(), true)
		machine.NextState(Forward)
	}
	for i := len(states) - 1; i >= 0; i-- {
		assert.Equal(machine.CurrentState(), states[i])
		assert.Equal(machine.Running(), true)
		machine.NextState(Backward)
	}
}
