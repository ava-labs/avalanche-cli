package cmd

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/models"
)

func writeGenesisFile(subnetName string, genesisBytes []byte) error {
	usr, _ := user.Current()
	genesisPath := filepath.Join(usr.HomeDir, BaseDir, subnetName+genesis_suffix)
	err := os.WriteFile(genesisPath, genesisBytes, 0644)
	return err
}

func copyGenesisFile(inputFilename string, subnetName string) error {
	genesisBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	usr, _ := user.Current()
	genesisPath := filepath.Join(usr.HomeDir, BaseDir, subnetName+genesis_suffix)
	err = os.WriteFile(genesisPath, genesisBytes, 0644)
	return err
}

func createSidecar(subnetName string, vm models.VmType) error {
	sc := models.Sidecar{
		Name:   subnetName,
		Vm:     vm,
		Subnet: subnetName,
	}

	scBytes, err := json.MarshalIndent(sc, "", "    ")
	if err != nil {
		return nil
	}

	usr, _ := user.Current()
	sidecarPath := filepath.Join(usr.HomeDir, BaseDir, subnetName+sidecar_suffix)
	err = os.WriteFile(sidecarPath, scBytes, 0644)
	return err
}

func loadSidecar(subnetName string) (models.Sidecar, error) {
	usr, _ := user.Current()
	sidecarPath := filepath.Join(usr.HomeDir, BaseDir, subnetName+sidecar_suffix)
	jsonBytes, err := os.ReadFile(sidecarPath)
	if err != nil {
		return models.Sidecar{}, err
	}

	var sc models.Sidecar
	err = json.Unmarshal(jsonBytes, &sc)
	if err != nil {
		return models.Sidecar{}, err
	}
	return sc, nil
}
