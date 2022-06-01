package vm

import (
	"math/big"

	"github.com/ava-labs/subnet-evm/params"
)

const GasLimit = 8000000

var Difficulty = big.NewInt(0)

var slowTarget = big.NewInt(15000000)
var mediumTarget = big.NewInt(20000000)
var fastTarget = big.NewInt(50000000)

// This is the current c-chain gas config
var StarterFeeConfig = params.FeeConfig{
	GasLimit:                 big.NewInt(8000000),
	MinBaseFee:               big.NewInt(25000000000),
	TargetGas:                big.NewInt(15000000),
	BaseFeeChangeDenominator: big.NewInt(36),
	MinBlockGasCost:          big.NewInt(0),
	MaxBlockGasCost:          big.NewInt(1000000),
	TargetBlockRate:          2,
	BlockGasCostStep:         big.NewInt(200000),
}
