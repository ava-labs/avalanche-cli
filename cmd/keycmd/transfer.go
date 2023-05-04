// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
)

const (
	sourceFlag          = "source"
	targetFlag          = "target"
	keyNameFlag         = "key"
	ledgerIndexFlag     = "ledger"
	wrongLedgerIndexVal = 32768
)

var (
	source      bool
	target      bool
	keyName     string
	ledgerIndex uint
	force       bool
)

func newTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "transfer addr amount",
		Short:        "Fund a ledger address or stored key from another one",
		Long:         `The key transfer command allows to transfer funds between stored keys or ledger addrs.`,
		RunE:         transferF,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
	}
	cmd.Flags().BoolVarP(
		&force,
		forceFlag,
		"f",
		false,
		"avoid transfer confirmation",
	)
	cmd.Flags().BoolVarP(
		&local,
		localFlag,
		"l",
		false,
		"transfer between local network addresses",
	)
	cmd.Flags().BoolVarP(
		&testnet,
		fujiFlag,
		"f",
		false,
		"transfer between testnet (fuji) addresses",
	)
	cmd.Flags().BoolVarP(
		&testnet,
		testnetFlag,
		"t",
		false,
		"transfer between testnet (fuji) addresses",
	)
	cmd.Flags().BoolVarP(
		&mainnet,
		mainnetFlag,
		"m",
		false,
		"transfer between mainnet addresses",
	)
	cmd.Flags().BoolVarP(
		&source,
		sourceFlag,
		"s",
		false,
		"do source transfer tx",
	)
	cmd.Flags().BoolVarP(
		&target,
		targetFlag,
		"g",
		false,
		"do target transfer tx",
	)
	cmd.Flags().StringVarP(
		&keyName,
		keyNameFlag,
		"k",
		"",
		"key to use for either source or target op",
	)
	cmd.Flags().UintVarP(
		&ledgerIndex,
		ledgerIndexFlag,
		"i",
		wrongLedgerIndexVal,
		"ledger index to use for either source or target op",
	)
	return cmd
}

func transferF(_ *cobra.Command, args []string) error {
	if (!source && !target) || (source && target) {
		return fmt.Errorf("one of %s, %s flags must be selected", sourceFlag, targetFlag)
	}
	amountStr := args[0]
	amountFlt, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return err
	}
	amount := uint64(amountFlt * float64(units.Avax))
	addrStr := args[1]
	addr, err := address.ParseToID(addrStr)
	if err != nil {
		return err
	}

	if (keyName == "" && ledgerIndex == wrongLedgerIndexVal) || (keyName != "" && ledgerIndex != wrongLedgerIndexVal) {
		return fmt.Errorf("only one between a keyname or a ledger index must be given")
	}

	var network models.Network
	if local {
		network = models.Local
	}
	if testnet {
		network = models.Fuji
	}
	if mainnet {
		network = models.Mainnet
	}
	if network == models.Undefined {
		// no flag was set, prompt user
		networkStr, err := app.Prompt.CaptureList(
			"Choose network in which to do the transfer",
			[]string{models.Mainnet.String(), models.Fuji.String(), models.Local.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	networkID, err := network.NetworkID()
	if err != nil {
		return err
	}

	fees := map[models.Network]uint64{
		models.Fuji:    genesis.FujiParams.TxFeeConfig.TxFee,
		models.Mainnet: genesis.MainnetParams.TxFeeConfig.TxFee,
		models.Local:   genesis.LocalParams.TxFeeConfig.TxFee,
	}
	fee := fees[network]

	var kc keychain.Keychain
	if keyName != "" {
		keyPath := app.GetKeyPath(keyName)
		sk, err := key.LoadSoft(networkID, keyPath)
		if err != nil {
			return err
		}
		kc = sk.KeyChain()
	} else {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return err
		}
		ledgerIndices := []uint32{uint32(ledgerIndex)}
		kc, err = keychain.NewLedgerKeychainFromIndices(ledgerDevice, ledgerIndices)
		if err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("this operation is going to:")
	if source {
		sourceAddr := kc.Addresses().List()[0]
		hrp := key.GetHRP(networkID)
		sourceAddrStr, err := address.Format("P", hrp, sourceAddr[:])
		if err != nil {
			return err
		}
		if sourceAddrStr == addrStr {
			return fmt.Errorf("source addr is the same as target addr")
		}
		ux.Logger.PrintToUser("- send %.9f AVAX from %s to target address %s", float64(amount)/float64(units.Avax), sourceAddrStr, addrStr)
		ux.Logger.PrintToUser("- take a fee of %.9f AVAX from source address %s", float64(4*fee)/float64(units.Avax), sourceAddrStr)
	} else {
		targetAddr := kc.Addresses().List()[0]
		hrp := key.GetHRP(networkID)
		targetAddrStr, err := address.Format("P", hrp, targetAddr[:])
		if err != nil {
			return err
		}
		if targetAddrStr != addrStr {
			return fmt.Errorf("target addr inconsistency: %s vs %s", targetAddrStr, addrStr)
		}
		ux.Logger.PrintToUser("- receive %.9f AVAX at target address %s", float64(amount)/float64(units.Avax), addrStr)
	}
	ux.Logger.PrintToUser("")

	if !force {
		confStr := "Confirm transfer"
		conf, err := app.Prompt.CaptureNoYes(confStr)
		if err != nil {
			return err
		}
		if !conf {
			ux.Logger.PrintToUser("Cancelled")
			return nil
		}
	}

	apiEndpoints := map[models.Network]string{
		models.Fuji:    constants.FujiAPIEndpoint,
		models.Mainnet: constants.MainnetAPIEndpoint,
		models.Local:   constants.LocalAPIEndpoint,
	}
	apiEndpoint := apiEndpoints[network]

	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{addr},
	}

	if source {
		wallet, err := primary.NewWalletWithTxs(context.Background(), apiEndpoint, kc)
		if err != nil {
			return err
		}
		output := &avax.TransferableOutput{
			Asset: avax.Asset{ID: wallet.P().AVAXAssetID()},
			Out: &secp256k1fx.TransferOutput{
				Amt:          amount + fee*3,
				OutputOwners: to,
			},
		}
		outputs := []*avax.TransferableOutput{output}
		ux.Logger.PrintToUser("P -> X source export")
		if _, err := wallet.P().IssueExportTx(wallet.X().BlockchainID(), outputs); err != nil {
			return err
		}
	} else {
		ux.Logger.PrintToUser("P -> X target import")
		wallet, err := primary.NewWalletWithTxs(context.Background(), apiEndpoint, kc)
		if err != nil {
			return err
		}
		if _, err = wallet.X().IssueImportTx(avago_constants.PlatformChainID, &to); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		output := &avax.TransferableOutput{
			Asset: avax.Asset{ID: wallet.P().AVAXAssetID()},
			Out: &secp256k1fx.TransferOutput{
				Amt:          amount + fee*1,
				OutputOwners: to,
			},
		}
		outputs := []*avax.TransferableOutput{output}
		ux.Logger.PrintToUser("X -> P target export")
		wallet, err = primary.NewWalletWithTxs(context.Background(), apiEndpoint, kc)
		if err != nil {
			return err
		}
		if _, err := wallet.X().IssueExportTx(avago_constants.PlatformChainID, outputs); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		wallet, err = primary.NewWalletWithTxs(context.Background(), apiEndpoint, kc)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("X -> P target import")
		if _, err = wallet.P().IssueImportTx(wallet.X().BlockchainID(), &to); err != nil {
			return err
		}
	}

	return nil
}
