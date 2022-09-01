// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package statemachine

type StateDirection int64

const (
	Forward StateDirection = iota
	Backward
	Stop
)

type StateMachine struct {
	index    int
	states   []string
	finished bool
}

func NewStateMachine(states []string) *StateMachine {
	return &StateMachine{
		states: states,
	}
}

func (sm *StateMachine) CurrentState() string {
	if sm.index < 0 || sm.index >= len(sm.states) {
		return ""
	}
	return sm.states[sm.index]
}

func (sm *StateMachine) NextState(direction StateDirection) string {
	switch direction {
	case Forward:
		sm.index++
	case Backward:
		sm.index--
	default:
		return ""
	}
	if sm.index == len(sm.states) {
		sm.Stop()
	}
	return sm.CurrentState()
}

func (sm *StateMachine) Running() bool {
	return !sm.finished
}

func (sm *StateMachine) Stop() {
	sm.finished = true
}
