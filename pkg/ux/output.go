// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/fatih/color"
)

var Logger *UserLog

type UserLog struct {
	Log    logging.Logger
	Writer io.Writer
}

func NewUserLog(log logging.Logger, userwriter io.Writer) {
	if Logger == nil {
		Logger = &UserLog{
			Log:    log,
			Writer: userwriter,
		}
	}
}

// PrintToUser prints msg directly on the screen, but also to Log file
func (ul *UserLog) PrintToUser(msg string, args ...interface{}) {
	fmt.Print("\r\033[K") // Clear the line from the cursor position to the end
	ul.print(fmt.Sprintf(msg, args...) + "\n")
}

func (ul *UserLog) print(msg string) {
	if ul != nil {
		fmt.Fprint(ul.Writer, msg)
		ul.Log.Info(msg)
	} else {
		fmt.Print(msg)
	}
}

// Info prints to the Log file
func (ul *UserLog) Info(msg string, args ...interface{}) {
	ul.Log.Info(fmt.Sprintf(msg, args...) + "\n")
}

// Error prints to the Log file
func (ul *UserLog) Error(msg string, args ...interface{}) {
	ul.Log.Error(fmt.Sprintf(msg, args...))
}

// GreenCheckmarkToUser prints a green checkmark to the user before the message
func (ul *UserLog) GreenCheckmarkToUser(msg string, args ...interface{}) {
	checkmark := "\u2713" // Unicode for checkmark symbol
	green := color.New(color.FgHiGreen).SprintFunc()
	ul.PrintToUser(green(checkmark)+" "+msg, args...)
}

func (ul *UserLog) RedXToUser(msg string, args ...interface{}) {
	xmark := "\u2717" // Unicode for X symbol
	red := color.New(color.FgHiRed).SprintFunc()
	ul.PrintToUser(red(xmark)+" "+msg, args...)
}

func (ul *UserLog) PrintLineSeparator() {
	ul.PrintToUser("==============================================")
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

func ConvertToStringWithThousandSeparator(input uint64) string {
	p := message.NewPrinter(language.English)
	s := p.Sprintf("%d", input)
	return strings.ReplaceAll(s, ",", "_")
}
