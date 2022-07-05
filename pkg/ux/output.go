// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
)

var Logger *UserLog

type UserLog struct {
	log    logging.Logger
	writer io.Writer
}

func NewUserLog(log logging.Logger, userwriter io.Writer) {
	if Logger == nil {
		Logger = &UserLog{
			log:    log,
			writer: userwriter,
		}
	}
}

// PrintToUser prints msg directly on the screen, but also to log file
func (ul *UserLog) PrintToUser(msg string, args ...interface{}) {
	fmt.Fprintln(ul.writer, fmt.Sprintf(msg, args...))
	ul.log.Info(msg, args...)
}

// PrintWait does some dot printing to entertain the user
func PrintWait(cancel chan struct{}) {
	for {
		select {
		case <-time.After(1 * time.Second):
			fmt.Print(".")
		case <-cancel:
			return
		}
	}
}
