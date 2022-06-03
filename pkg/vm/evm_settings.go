package vm

import (
	"math/big"

	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
)

const (
	GasLimit = 8000000

	defaultAirdropAmount = "1000000000000000000000000"
)

var (
	Difficulty = big.NewInt(0)

	slowTarget   = big.NewInt(15000000)
	mediumTarget = big.NewInt(20000000)
	fastTarget   = big.NewInt(50000000)

	// This is the current c-chain gas config
	StarterFeeConfig = params.FeeConfig{
		GasLimit:                 big.NewInt(8000000),
		MinBaseFee:               big.NewInt(25000000000),
		TargetGas:                big.NewInt(15000000),
		BaseFeeChangeDenominator: big.NewInt(36),
		MinBlockGasCost:          big.NewInt(0),
		MaxBlockGasCost:          big.NewInt(1000000),
		TargetBlockRate:          2,
		BlockGasCostStep:         big.NewInt(200000),
	}

	ewokAddress = common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")
	oneAvax     = big.NewInt(1000000000000000000)
)
