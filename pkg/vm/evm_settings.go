package vm

import (
	"math/big"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
)

const (
	GasLimit = 8_000_000

	defaultAirdropAmount = "1000000000000000000000000"
)

var (
	Difficulty = big.NewInt(0)

	slowTarget   = big.NewInt(15_000_000)
	mediumTarget = big.NewInt(20_000_000)
	fastTarget   = big.NewInt(50_000_000)

	// This is the current c-chain gas config
	StarterFeeConfig = params.FeeConfig{
		GasLimit:                 big.NewInt(8_000_000),
		MinBaseFee:               big.NewInt(25_000_000_000),
		TargetGas:                big.NewInt(15_000_000),
		BaseFeeChangeDenominator: big.NewInt(36),
		MinBlockGasCost:          big.NewInt(0),
		MaxBlockGasCost:          big.NewInt(1_000_000),
		TargetBlockRate:          2,
		BlockGasCostStep:         big.NewInt(200_000),
	}

	Prefunded_ewoq_Address = common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")

	oneAvax = new(big.Int).SetUint64(units.Avax)
)
