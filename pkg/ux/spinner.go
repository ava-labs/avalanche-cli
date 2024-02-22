// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"

	"github.com/chelnak/ysmrr"
	"github.com/chelnak/ysmrr/pkg/animations"
	"github.com/chelnak/ysmrr/pkg/colors"
)

var Spinner *UserSpinner

type UserSpinner struct {
	spinner ysmrr.SpinnerManager
	stopped bool
}

func newSpinner() ysmrr.SpinnerManager {
	return ysmrr.NewSpinnerManager(
		ysmrr.WithAnimation(animations.Dots),
		ysmrr.WithSpinnerColor(colors.FgHiBlue),
	)
}

func NewUserSpinner() *UserSpinner {
	spinner := newSpinner()
	Spinner = &UserSpinner{spinner: spinner, stopped: true}
	return Spinner
}

func (us *UserSpinner) Start() {
	if us.stopped {
		us.spinner.Start()
	}
}

func (us *UserSpinner) Stop() {
	if !us.stopped {
		us.spinner.Stop()
	}
}

func (us *UserSpinner) SpinToUser(msg string, args ...interface{}) *ysmrr.Spinner {
	formattedMsg := fmt.Sprintf(msg, args...)
	sp := us.spinner.AddSpinner(formattedMsg)
	us.Start()
	return sp
}

func SpinFailWithError(s *ysmrr.Spinner, txt string, err error) {
	if txt == "" {
		s.UpdateMessage(fmt.Sprintf("%s err:%v", s.GetMessage(), err))
	} else {
		s.UpdateMessage(fmt.Sprintf("%s txt:%s err:%v", s.GetMessage(), txt, err))
	}
	s.Error()
}

func SpinComplete(s *ysmrr.Spinner) {
	s.Complete()
}
