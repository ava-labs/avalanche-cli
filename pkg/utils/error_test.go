// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bytes"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

func TestFindErrorLogs(t *testing.T) {
	/*
			expectedOutput := `================================= !!! ================================
		Found some error strings in the logs, check these for possible causes:
		[01-17|23:20:13.003] ERROR server/server.go:198 invented this log for the test
		[01-17|23:20:13.003] DEBUG server/server.go:198 This is a DEBUG log but contains the word *error* so it should be shown
		[01-17|23:20:13.003] WARN server/server.go:188 root context is done
		[01-17|23:20:13.003] WARN server/server.go:191 closed gRPC gateway server
		[01-17|23:20:13.003] WARN server/server.go:196 closed gRPC server
		[01-17|23:20:13.003] WARN server/server.go:198 gRPC terminated
		================================= !!! ================================
		`
	*/
	expectedOutput := `================================= !!! ================================
Found some error strings in the logs, check these for possible causes:
----------------------------------------------------------------------
-- Found error logs in file at path ./error_test_log_file.log:
[01-17|23:20:12.996] WARN server/server.go:388 async call failed to complete {"async-call": "waiting for local cluster readiness", "error": "node \"node5\" stopped unexpectedly"}
----------------------------------------------------------------------
[01-17|23:20:13.003] ERROR server/server.go:198 invented this log for the test 
----------------------------------------------------------------------
================!!! end of errors in logs !!! ========================
`

	require := require.New(t)

	underlyingBuffer := make([]byte, 0, len(expectedOutput))
	buf := bytes.NewBuffer(underlyingBuffer)

	ux.NewUserLog(logging.NoLog{}, buf)
	FindErrorLogs("./error_test_log_file.log")

	require.Equal(expectedOutput, buf.String())
}
