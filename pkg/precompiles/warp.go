// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package precompiles

import (
	_ "embed"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanchego/ids"
)

func WarpPrecompileGetBlockchainID(
	rpcURL string,
) (ids.ID, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		WarpPrecompile,
		"getBlockchainID()->(bytes32)",
	)
	if err != nil {
		return ids.Empty, err
	}
	received, b := out[0].([32]byte)
	if !b {
		return ids.Empty, fmt.Errorf("error at getBlockchainID call, expected ids.ID, got %T", out[0])
	}
	return received, nil
}
