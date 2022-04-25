/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"os"
)

func createCustomGenesis(name string) ([]byte, error) {
	fmt.Println("creating custom VM subnet", name)

	genesisPath, err := captureExistingFilepath("Enter path to custom genesis")
	if err != nil {
		return []byte{}, err
	}

	genesisBytes, err := os.ReadFile(genesisPath)
	return genesisBytes, err
}
