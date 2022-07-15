package keycmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/olekukonko/tablewriter"
)

func printAddresses(keyPaths []string) error {
	header := []string{"Key Name", "Chain", "Address", "Network"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAutoMergeCells(true)

	supportedNetworks := map[string]uint32{
		models.Local.String(): 0,
		models.Fuji.String():  avago_constants.FujiID,
		/*
			Not enabled yet
			models.Mainnet.String(): avago_constants.MainnetID,
		*/
	}
	for _, keyPath := range keyPaths {
		cAdded := false
		keyName := strings.TrimSuffix(filepath.Base(keyPath), constants.KeySuffix)
		for net, id := range supportedNetworks {
			sk, err := key.LoadSoft(id, keyPath)
			if err != nil {
				return err
			}
			if !cAdded {
				strC := sk.C()
				table.Append([]string{keyName, "C-Chain (Ethereum hex format)", strC, "All"})
			}
			cAdded = true

			strP := sk.P()
			for _, p := range strP {
				table.Append([]string{keyName, "P-Chain (Bech32 format)", p, net})
			}
		}
	}

	table.Render()
	return nil
}
