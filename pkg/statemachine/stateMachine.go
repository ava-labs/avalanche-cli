// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package statemachine

import "errors"

type StateDirection int64

const (
	Forward StateDirection = iota
	Backward
	Stop
)

const notRunningState = ""

// keeps track of a lineal state sequence given by the non empty slice [states], which can
// be updated with steps forward and backward by using NextState() with suitable direction.
// starts in the first elem of [states] and ends either when Stop() is called,
// or when state bypasses last elem of [states]
//
// usage example:
//
//   machine, err := NewStateMachine(states)
//   [ err processing ]
//   while machine.Running() {
//     [do stuff based on machine.CurrentState(), and set direction]
//     machine.NextState(direction)
//   }
type StateMachine struct {
	index    int
	states   []string
	finished bool
}

func NewStateMachine(states []string) (*StateMachine, error) {
	if len(states) == 0 {
		return nil, errors.New("number of states must be greater than zero")
	}
	return &StateMachine{
		states: states,
	}, nil
}

func (sm *StateMachine) CurrentState() string {
	if !sm.Running() {
		return notRunningState
	}
	return sm.states[sm.index]
}

func (sm *StateMachine) NextState(direction StateDirection) {
	if !sm.Running() {
		return
	}
	switch direction {
	case Backward:
		sm.index--
		if sm.index < 0 {
			sm.index = 0
		}
	case Forward:
		sm.index++
		if sm.index == len(sm.states) {
			sm.Stop()
		}
	default:
		sm.Stop()
	}
}

func (sm *StateMachine) Running() bool {
	return !sm.finished
}

func (sm *StateMachine) Stop() {
	sm.finished = true
}
