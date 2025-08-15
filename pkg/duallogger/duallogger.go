// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package duallogger

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
)

// DualLogger outputs to both terminal (via ux.Logger) and log files (via app.Log)
type DualLogger struct {
	appLogger     logging.Logger
	CliOutputOnly bool
}

func (h *DualLogger) Write(p []byte) (n int, err error) {
	// Write to both terminal and log file
	return h.appLogger.Write(p)
}

func (h *DualLogger) WithOptions(opts ...zap.Option) logging.Logger {
	// Return a new logger with the options applied
	return h.appLogger.WithOptions(opts...)
}

func NewDualLogger(cliOutputOnly bool, app *application.Avalanche) *DualLogger {
	return &DualLogger{
		appLogger:     app.Log,
		CliOutputOnly: cliOutputOnly,
	}
}

func (h *DualLogger) Info(msg string, _ ...zap.Field) {
	ux.Logger.PrintToUser(msg)
	if !h.CliOutputOnly {
		h.appLogger.Info(msg)
	}
}

func (h *DualLogger) Error(msg string, _ ...zap.Field) {
	ux.Logger.RedXToUser(msg)
	if !h.CliOutputOnly {
		h.appLogger.Error(fmt.Sprintf("error fucn %s", msg))
	}
}

func (h *DualLogger) Debug(msg string, _ ...zap.Field) {
	ux.Logger.PrintToUser(msg)
	if !h.CliOutputOnly {
		h.appLogger.Debug(msg)
	}
}

func (h *DualLogger) Warn(msg string, _ ...zap.Field) {
	ux.Logger.PrintToUser(logging.Yellow.Wrap(msg))
	if !h.CliOutputOnly {
		h.appLogger.Warn(msg)
	}
}

func (h *DualLogger) Fatal(msg string, _ ...zap.Field) {
	ux.Logger.RedXToUser(msg)
	if !h.CliOutputOnly {
		h.appLogger.Fatal(msg)
	}
}

func (h *DualLogger) Verbo(msg string, _ ...zap.Field) {
	ux.Logger.PrintToUser(msg)
	if !h.CliOutputOnly {
		h.appLogger.Verbo(msg)
	}
}

func (h *DualLogger) Enabled(level logging.Level) bool {
	return h.appLogger.Enabled(level)
}

func (h *DualLogger) RecoverAndExit(exit func(), panicHandler func()) {
	h.appLogger.RecoverAndExit(exit, panicHandler)
}

func (h *DualLogger) RecoverAndPanic(panicHandler func()) {
	h.appLogger.RecoverAndPanic(panicHandler)
}

func (h *DualLogger) SetLevel(level logging.Level) {
	h.appLogger.SetLevel(level)
}

func (h *DualLogger) Stop() {
	h.appLogger.Stop()
}

func (h *DualLogger) StopOnPanic() {
	h.appLogger.StopOnPanic()
}

func (h *DualLogger) Trace(msg string, _ ...zap.Field) {
	ux.Logger.PrintToUser(logging.Blue.Wrap(msg))
	h.appLogger.Trace(msg)
}

func (h *DualLogger) With(fields ...zap.Field) logging.Logger {
	return h.appLogger.With(fields...)
}
