// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cloud

const (
	// Running state of the cloud item
	Running = State("running")

	// Stopped state of the cloud item
	Stopped = State("stopped")

	// Terminated state of the cloud item
	Terminated = State("terminated")

	// Unused state of the cloud item
	Unused = State("unused")

	// InUse state of the cloud item
	InUse = State("in-use")

	// Failed state of the cloud item
	Failed = State("failed")

	// Unknown state of the cloud item
	Unknown = State("unknown")
)

// State string representation of the cloud item
type State string
