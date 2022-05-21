// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"

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
func (ul *UserLog) PrintToUser(msg string) {
	fmt.Fprintln(ul.writer, msg)
	ul.log.Info(msg)
}
