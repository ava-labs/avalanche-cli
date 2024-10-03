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
	require.ErrorIs(err, errNoStates)
	require.Nil(machine)
}

func Test_StateMachineStop(t *testing.T) {
	require := require.New(t)

	machine, err := NewStateMachine([]string{"state"})
	require.NoError(err)
	require.Equal("state", machine.CurrentState())
	require.True(machine.Running())
	machine.Stop()
	require.Equal(notRunningState, machine.CurrentState())
	require.False(machine.Running())
}

func Test_StateMachineOneState(t *testing.T) {
	require := require.New(t)

	machine, err := NewStateMachine([]string{"state"})
	require.NoError(err)
	require.Equal("state", machine.CurrentState())
	require.True(machine.Running())
	machine.NextState(Forward)
	require.Equal(notRunningState, machine.CurrentState())
	require.False(machine.Running())
}

func Test_StateMachineNStates(t *testing.T) {
	require := require.New(t)

	states := []string{"one", "two", "...", "N"}
	machine, err := NewStateMachine(states)
	require.NoError(err)
	for _, state := range states {
		require.Equal(machine.CurrentState(), state)
		require.True(machine.Running())
		machine.NextState(Forward)
	}
	require.Equal(notRunningState, machine.CurrentState())
	require.False(machine.Running())
}

func Test_StateMachineBackward(t *testing.T) {
	require := require.New(t)

	states := []string{"one", "two", "...", "N"}
	machine, err := NewStateMachine(states)
	require.NoError(err)
	machine.NextState(Backward)
	require.Equal(machine.CurrentState(), states[0])
	require.True(machine.Running())
	for i := 0; i < len(states)-1; i++ {
		require.Equal(machine.CurrentState(), states[i])
		require.True(machine.Running())
		machine.NextState(Forward)
		require.Equal(machine.CurrentState(), states[i+1])
		require.True(machine.Running())
		machine.NextState(Backward)
		require.Equal(machine.CurrentState(), states[i])
		require.True(machine.Running())
		machine.NextState(Forward)
	}
	for i := len(states) - 1; i >= 0; i-- {
		require.Equal(machine.CurrentState(), states[i])
		require.True(machine.Running())
		machine.NextState(Backward)
	}
}
