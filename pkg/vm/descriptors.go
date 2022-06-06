package vm

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
)

func getChainId() (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	fmt.Println("Enter your subnet's ChainId. It can be any positive integer.")

	chainId, err := prompts.CapturePositiveBigInt("ChainId")
	if err != nil {
		return nil, err
	}

	return chainId, nil
}
