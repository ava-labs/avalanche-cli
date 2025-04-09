// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/ethereum/go-ethereum/crypto"
)

var ErrUnknownErrorSelector = fmt.Errorf("unknown error selector")

// also used at mocks
var (
	rpcDialContext = rpc.DialContext
)

// wraps over rpc.Client for calls used by SDK. used to make evm calls not available in ethclient:
// - debug trace call
// - debug trace transaction
// features:
// - finds out url scheme in case it is missing, to connect to ws/wss/http/https
// - repeats to try to recover from failures, generating its own context for each call
// - logs rpc url in case of failure
type RawClient struct {
	RPCClient *rpc.Client
	URL       string
}

// connects a raw evm rpc client to the given [rpcURL]
// supports [repeatsOnFailure] failures
func GetRawClient(rpcURL string) (RawClient, error) {
	client := RawClient{
		URL: rpcURL,
	}
	hasScheme, err := HasScheme(rpcURL)
	if err != nil {
		return RawClient{}, err
	}
	client.RPCClient, err = utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (*rpc.Client, error) {
			if hasScheme {
				return rpcDialContext(ctx, rpcURL)
			} else {
				_, scheme, err := GetClientWithoutScheme(rpcURL)
				if err != nil {
					return nil, err
				}
				return rpcDialContext(ctx, scheme+rpcURL)
			}
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure connecting to rpc client on %s: %w", rpcURL, err)
	}
	return client, err
}

// closes underlying rpc connection
func (client RawClient) Close() {
	client.RPCClient.Close()
}

// returns a trace for the given [txID] on [client]
// supports [repeatsOnFailure] failures
func (client RawClient) DebugTraceTransaction(
	txID string,
) (map[string]interface{}, error) {
	trace, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (map[string]interface{}, error) {
			var trace map[string]interface{}
			err := client.RPCClient.CallContext(
				ctx,
				&trace,
				"debug_traceTransaction",
				txID,
				map[string]string{"tracer": "callTracer"},
			)
			return trace, err
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure tracing tx %s on %s: %w", txID, client.URL, err)
	}
	return trace, err
}

// returns a trace for making a call on [client] with the given [data]
// supports [repeatsOnFailure] failures
func (client RawClient) DebugTraceCall(
	data map[string]string,
) (map[string]interface{}, error) {
	trace, err := utils.RetryWithContextGen(
		utils.GetAPILargeContext,
		func(ctx context.Context) (map[string]interface{}, error) {
			var trace map[string]interface{}
			err := client.RPCClient.CallContext(
				ctx,
				&trace,
				"debug_traceCall",
				data,
				"latest",
				map[string]interface{}{
					"tracer": "callTracer",
					"tracerConfig": map[string]interface{}{
						"onlyTopCall": false,
					},
				},
			)
			return trace, err
		},
		repeatsOnFailure,
		sleepBetweenRepeats,
	)
	if err != nil {
		err = fmt.Errorf("failure tracing call on %s: %w", client.URL, err)
	}
	return trace, err
}

// returns a trace for the given [txID] on [rpcURL]
// supports [repeatsOnFailure] failures
func GetTxTrace(rpcURL string, txID string) (map[string]interface{}, error) {
	client, err := GetRawClient(rpcURL)
	if err != nil {
		return nil, err
	}
	return client.DebugTraceTransaction(txID)
}

// returns evm function selector code for the given function signature
// evm maps function and error signatures into codes that are then used in traces
func GetFunctionSelector(functionSignature string) string {
	return "0x" + hex.EncodeToString(crypto.Keccak256([]byte(functionSignature))[:4])
}

// returns golang error associated with [trace] by using [functionSignatureToError]
// to map function signatures to evm function selectors in [trace], and then to golang errors
// first returned error is the mapped error, second error is for errors obtained
// executing this function
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
