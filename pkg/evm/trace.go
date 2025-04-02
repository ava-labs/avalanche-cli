// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/ethereum/go-ethereum/crypto"
)

var ErrUnknownErrorSelector = fmt.Errorf("unknown error selector")

func GetRPCClient(rpcURL string) (*rpc.Client, error) {
	var (
		client *rpc.Client
		err    error
	)
	hasScheme, err := HasScheme(rpcURL)
	if err != nil {
		return nil, err
	}
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		if !hasScheme {
			_, scheme, findErr := GetClientWithoutScheme(rpcURL)
			if findErr == nil {
				client, err = rpc.DialContext(ctx, scheme+rpcURL)
			} else {
				err = findErr
			}
		} else {
			client, err = rpc.DialContext(ctx, rpcURL)
		}
		if err == nil {
			break
		}
		err = fmt.Errorf("failure connecting to rpc client on %s: %w", rpcURL, err)
		ux.Logger.RedXToUser("%s", err)
		time.Sleep(sleepBetweenRepeats)
	}
	return client, err
}

func DebugTraceTransaction(
	client *rpc.Client,
	txID string,
) (map[string]interface{}, error) {
	var (
		err   error
		trace map[string]interface{}
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		err = client.CallContext(
			ctx,
			&trace,
			"debug_traceTransaction",
			txID,
			map[string]string{"tracer": "callTracer"},
		)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure tracing tx %s for client %#v: %w", txID, client, err)
		time.Sleep(sleepBetweenRepeats)
	}
	return trace, err
}

func GetTrace(rpcURL string, txID string) (map[string]interface{}, error) {
	client, err := GetRPCClient(rpcURL)
	if err != nil {
		return nil, err
	}
	return DebugTraceTransaction(client, txID)
}

func DebugTraceCall(
	client *rpc.Client,
	toTrace map[string]string,
) (map[string]interface{}, error) {
	var (
		err   error
		trace map[string]interface{}
	)
	for i := 0; i < repeatsOnFailure; i++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		err = client.CallContext(
			ctx,
			&trace,
			"debug_traceCall",
			toTrace,
			"latest",
			map[string]interface{}{
				"tracer": "callTracer",
				"tracerConfig": map[string]interface{}{
					"onlyTopCall": false,
				},
			},
		)
		if err == nil {
			break
		}
		err = fmt.Errorf("failure tracing call for client %#v: %w", client, err)
		time.Sleep(sleepBetweenRepeats)
	}
	return trace, err
}

func GetFunctionSelector(functionSignature string) string {
	return "0x" + hex.EncodeToString(crypto.Keccak256([]byte(functionSignature))[:4])
}

func GetErrorFromTrace(
	trace map[string]interface{},
	functionSignatureToError map[string]error,
) (error, error) {
	traceOutputI, ok := trace["output"]
	if !ok {
		return nil, fmt.Errorf("trace does not contain output field")
	}
	traceOutput, ok := traceOutputI.(string)
	if !ok {
		return nil, fmt.Errorf("expected type string for trace output, got %T", traceOutputI)
	}
	traceOutputBytes, err := hex.DecodeString(strings.TrimPrefix(traceOutput, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failure decoding trace output: %w", err)
	}
	if len(traceOutputBytes) < 4 {
		return nil, fmt.Errorf("less than 4 bytes in trace output")
	}
	traceErrorSelector := "0x" + hex.EncodeToString(traceOutputBytes[:4])
	for errorSignature, err := range functionSignatureToError {
		errorSelector := GetFunctionSelector(errorSignature)
		if traceErrorSelector == errorSelector {
			return err, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownErrorSelector, traceErrorSelector)
}
