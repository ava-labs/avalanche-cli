// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package statemachine

import (
	"errors"
)

type StateDirection int64

const (
	Forward StateDirection = iota
	Backward
	Stop
)

type stateMachine struct {
	index  int
	states []string
}

func NewStateMachine(states []string) *stateMachine {
	return &stateMachine{
		states: states,
	}
}

func (sm *stateMachine) CurrentState() (string, error) {
	if sm.index < 0 || sm.index >= len(sm.states) {
		return "", errors.New("invalid state machine index")
	}
	return sm.states[sm.index], nil
}

func (sm *stateMachine) NextState(direction StateDirection) (string, error) {
	switch direction {
	case Forward:
		sm.index++
	case Backward:
		sm.index--
	default:
		return "", errors.New("invalid state machine direction")
	}
	return sm.CurrentState()
}
