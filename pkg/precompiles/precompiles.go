// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package precompiles

import (
	"github.com/ethereum/go-ethereum/common"

	_ "embed"
)

var NativeMinterPrecompile = common.HexToAddress("0x0200000000000000000000000000000000000001")
