// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statemachine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_StateMachineNoStates(t *testing.T) {
	require := require.New(t)

	machine, err := NewStateMachine([]string{})
	require.Error(err)
	require.Equal(machine, (*StateMachine)(nil))
}

func Test_StateMachineStop(t *testing.T) {
	require := require.New(t)

	machine, err := NewStateMachine([]string{"state"})
	require.NoError(err)
	require.Equal(machine.CurrentState(), "state")
	require.Equal(machine.Running(), true)
	machine.Stop()
	require.Equal(machine.CurrentState(), notRunningState)
	require.Equal(machine.Running(), false)
}

func Test_StateMachineOneState(t *testing.T) {
	require := require.New(t)

	machine, err := NewStateMachine([]string{"state"})
	require.NoError(err)
	require.Equal(machine.CurrentState(), "state")
	require.Equal(machine.Running(), true)
	machine.NextState(Forward)
	require.Equal(machine.CurrentState(), notRunningState)
	require.Equal(machine.Running(), false)
}

func Test_StateMachineNStates(t *testing.T) {
	require := require.New(t)

	states := []string{"one", "two", "...", "N"}
	machine, err := NewStateMachine(states)
	require.NoError(err)
	for _, state := range states {
		require.Equal(machine.CurrentState(), state)
		require.Equal(machine.Running(), true)
		machine.NextState(Forward)
	}
	require.Equal(machine.CurrentState(), notRunningState)
	require.Equal(machine.Running(), false)
}

func Test_StateMachineBackward(t *testing.T) {
	require := require.New(t)

	states := []string{"one", "two", "...", "N"}
	machine, err := NewStateMachine(states)
	require.NoError(err)
	machine.NextState(Backward)
	require.Equal(machine.CurrentState(), states[0])
	require.Equal(machine.Running(), true)
	for i := 0; i < len(states)-1; i++ {
		require.Equal(machine.CurrentState(), states[i])
		require.Equal(machine.Running(), true)
		machine.NextState(Forward)
		require.Equal(machine.CurrentState(), states[i+1])
		require.Equal(machine.Running(), true)
		machine.NextState(Backward)
		require.Equal(machine.CurrentState(), states[i])
		require.Equal(machine.Running(), true)
		machine.NextState(Forward)
	}
	for i := len(states) - 1; i >= 0; i-- {
		require.Equal(machine.CurrentState(), states[i])
		require.Equal(machine.Running(), true)
		machine.NextState(Backward)
	}
}
