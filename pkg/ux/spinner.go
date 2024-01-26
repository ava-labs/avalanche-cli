// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"

	"github.com/chelnak/ysmrr"
	"github.com/chelnak/ysmrr/pkg/animations"
	"github.com/chelnak/ysmrr/pkg/colors"
)

var Spinner ysmrr.SpinnerManager

type UserSpinner struct {
	spinner ysmrr.SpinnerManager
}

func NewUserSpinner() {
	if Spinner == nil {
		Spinner = ysmrr.NewSpinnerManager(
			ysmrr.WithAnimation(animations.Dots),
			ysmrr.WithSpinnerColor(colors.FgHiBlue),
		)
		Spinner.Start()
	}
}

// SpinToUser adds spinner to the screen
func (us *UserSpinner) SpinToUser(msg string, args ...interface{}) *ysmrr.Spinner {
	formattedMsg := fmt.Sprintf(msg, args...)
	return us.spinner.AddSpinner(formattedMsg)
}

func (us *UserSpinner) FailWithError(s *ysmrr.Spinner, err error) {
	s.UpdateMessage(s.GetMessage() + " err: " + err.Error())
	s.Error()
}

func (us *UserSpinner) Done(s *ysmrr.Spinner) {
	us.spinner.Stop()
	us.spinner = nil
}
