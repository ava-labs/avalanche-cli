/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package vm

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
)

func CreateCustomGenesis(name string) ([]byte, error) {
	fmt.Println("creating custom VM subnet", name)

	genesisPath, err := prompts.CaptureExistingFilepath("Enter path to custom genesis")
	if err != nil {
		return []byte{}, err
	}

	genesisBytes, err := os.ReadFile(genesisPath)
	return genesisBytes, err
}
