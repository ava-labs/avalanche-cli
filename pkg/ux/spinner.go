// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/chelnak/ysmrr"
	"github.com/chelnak/ysmrr/pkg/animations"
	"github.com/chelnak/ysmrr/pkg/colors"
)

type UserSpinner struct {
	spinner    ysmrr.SpinnerManager
	started    bool
	mutex      sync.Mutex
	startTimes map[*ysmrr.Spinner]time.Time
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
	spinner := &UserSpinner{
		spinner:    newSpinner(nil),
		mutex:      sync.Mutex{},
		startTimes: make(map[*ysmrr.Spinner]time.Time),
	}
	return spinner
}

func (us *UserSpinner) Stop() {
	us.mutex.Lock()
	us.spinner.Stop()
	us.mutex.Unlock()
}

func (us *UserSpinner) SpinToUser(msg string, args ...interface{}) *ysmrr.Spinner {
	formattedMsg := fmt.Sprintf(msg, args...)
	Logger.log.Info(formattedMsg + " [Spinner Start]")
	sp := us.spinner.AddSpinner(formattedMsg)
	us.mutex.Lock()
	if !us.started {
		us.spinner.Start()
		us.started = true
	}
	us.startTimes[sp] = time.Now()
	us.mutex.Unlock()
	return sp
}

func (us *UserSpinner) SpinFailWithError(s *ysmrr.Spinner, txt string, err error) {
	elapsed := 0 * time.Second
	if startTime, ok := us.startTimes[s]; ok {
		elapsed = time.Since(startTime)
	}
	if txt == "" {
		s.UpdateMessage(fmt.Sprintf("%s err:%v [%.1fs]", s.GetMessage(), err, elapsed.Seconds()))
	} else {
		s.UpdateMessage(fmt.Sprintf("%s txt:%s err:%v [%.1fs]", s.GetMessage(), txt, err, elapsed.Seconds()))
	}
	s.Error()
	Logger.log.Info(s.GetMessage() + " [Spinner Err]")
}

func (us *UserSpinner) SpinComplete(s *ysmrr.Spinner) {
	if s.IsComplete() {
		return
	}
	elapsed := 0 * time.Second
	if startTime, ok := us.startTimes[s]; ok {
		elapsed = time.Since(startTime)
	}
	s.UpdateMessage(fmt.Sprintf("%s [%.1fs]", s.GetMessage(), elapsed.Seconds()))

	s.Complete()
	Logger.log.Info(s.GetMessage() + " [Spinner Complete]")
}
