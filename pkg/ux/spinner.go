// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/chelnak/ysmrr"
	"github.com/chelnak/ysmrr/pkg/animations"
	"github.com/chelnak/ysmrr/pkg/colors"
)

type UserSpinner struct {
	spinner ysmrr.SpinnerManager
	mutex   sync.Mutex
}

func newSpinner(writer io.Writer) ysmrr.SpinnerManager {
	if writer == nil {
		writer = os.Stdout
	}
	return ysmrr.NewSpinnerManager(
		ysmrr.WithAnimation(animations.Dots),
		ysmrr.WithSpinnerColor(colors.FgHiBlue),
		ysmrr.WithWriter(writer),
	)
}

func NewUserSpinner() *UserSpinner {
	spinner := &UserSpinner{spinner: newSpinner(nil), mutex: sync.Mutex{}}
	return spinner
}

func (us *UserSpinner) Start() {
	us.mutex.Lock()
	us.spinner.Start()
	us.mutex.Unlock()
}

func (us *UserSpinner) Stop() {
	us.mutex.Lock()
	us.spinner.Stop()
	us.mutex.Unlock()
}

func (us *UserSpinner) SpinToUser(msg string, args ...interface{}) *ysmrr.Spinner {
	us.mutex.Lock()
	formattedMsg := fmt.Sprintf(msg, args...)
	sp := us.spinner.AddSpinner(formattedMsg)
	us.spinner.Start()
	us.mutex.Unlock()
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
