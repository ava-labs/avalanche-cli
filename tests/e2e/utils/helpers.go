package utils

import (
	"errors"
	"os"
	"os/user"
	"path"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func GetBaseDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return path.Join(usr.HomeDir, baseDir)
}

func SubnetConfigExists(subnetName string) (bool, error) {
	genesis := path.Join(GetBaseDir(), subnetName+constants.Genesis_suffix)
	genesisExists := true
	if _, err := os.Stat(genesis); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		genesisExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}

	sidecar := path.Join(GetBaseDir(), subnetName+constants.Sidecar_suffix)
	sidecarExists := true
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		sidecarExists = false
	} else if err != nil {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}

	// do an xor
	if (genesisExists || sidecarExists) && !(genesisExists && sidecarExists) {
		return false, errors.New("config half exists")
	}
	return genesisExists && sidecarExists, nil
}

func ParseRPCFromOutput(output string) (string, error) {
	// split output by newline
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "RPC URL:") {
			index := strings.Index(line, "http")
			if index == -1 {
				return "", errors.New("no url in RPC URL line")
			}
			return line[index:], nil
		}
	}
	return "", errors.New("no rpc url found")
}
