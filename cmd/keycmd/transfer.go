// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/spf13/cobra"
)

const (
	sendFlag                = "send"
	receiveFlag             = "receive"
	keyNameFlag             = "key"
	ledgerIndexFlag         = "ledger"
	receiverAddrFlag        = "target-addr"
	amountFlag              = "amount"
	wrongLedgerIndexVal     = 32768
	receiveRecoveryStepFlag = "receive-recovery-step"
)

var (
	send                bool
	receive             bool
	keyName             string
	ledgerIndex         uint32
	force               bool
	receiverAddrStr     string
	amountFlt           float64
	receiveRecoveryStep uint64
)

func newTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "transfer [options]",
		Short:        "Fund a ledger address or stored key from another one",
		Long:         `The key transfer command allows to transfer funds between stored keys or ledger addrs.`,
		RunE:         transferF,
		Args:         cobra.ExactArgs(0),
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
		"u",
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
		&send,
		sendFlag,
		"s",
		false,
		"send the transfer",
	)
	cmd.Flags().BoolVarP(
		&receive,
		receiveFlag,
		"g",
		false,
		"receive the transfer",
	)
	cmd.Flags().StringVarP(
		&keyName,
		keyNameFlag,
		"k",
		"",
		"key associated to the sender or receiver address",
	)
	cmd.Flags().Uint32VarP(
		&ledgerIndex,
		ledgerIndexFlag,
		"i",
		wrongLedgerIndexVal,
		"ledger index associated to the sender or receiver address",
	)
	cmd.Flags().Uint64VarP(
		&receiveRecoveryStep,
		receiveRecoveryStepFlag,
		"r",
		0,
		"receive step to use for multiple step transaction recovery",
	)
	cmd.Flags().StringVarP(
		&receiverAddrStr,
		receiverAddrFlag,
		"a",
		"",
		"receiver address",
	)
	cmd.Flags().Float64VarP(
		&amountFlt,
		amountFlag,
		"o",
		0,
		"amount to send or receive (AVAX units)",
	)
	return cmd
}

func transferF(*cobra.Command, []string) error {
	if send && receive {
		return fmt.Errorf("only one of %s, %s flags should be selected", sendFlag, receiveFlag)
	}

	if keyName != "" && ledgerIndex != wrongLedgerIndexVal {
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
			"Network to use",
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

	if !send && !receive {
		option, err := app.Prompt.CaptureList(
			"Step of the transfer",
			[]string{"Send", "Receive"},
		)
		if err != nil {
			return err
		}
		if option == "Send" {
			send = true
		} else {
			receive = true
		}
	}

	if keyName == "" && ledgerIndex == wrongLedgerIndexVal {
		var useLedger bool
		goalStr := ""
		if send {
			goalStr = " for the sender address"
		} else {
			goalStr = " for the receiver address"
		}
		useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, goalStr, app.GetKeyDir())
		if err != nil {
			return err
		}
		if useLedger {
			ledgerIndex, err = app.Prompt.CaptureUint32("Ledger index to use")
			if err != nil {
				return err
			}
		}
	}

	if amountFlt == 0 {
		var promptStr string
		if send {
			promptStr = "Amount to send (AVAX units)"
		} else {
			promptStr = "Amount to receive (AVAX units)"
		}
		amountFlt, err = app.Prompt.CaptureFloat(promptStr, func(v float64) error {
			if v <= 0 {
				return fmt.Errorf("value %f must be greater than zero", v)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	amount := uint64(amountFlt * float64(units.Avax))

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
		ledgerIndices := []uint32{ledgerIndex}
		kc, err = keychain.NewLedgerKeychainFromIndices(ledgerDevice, ledgerIndices)
		if err != nil {
			return err
		}
	}

	var receiverAddr ids.ShortID
	if send {
		if receiverAddrStr == "" {
			receiverAddrStr, err = app.Prompt.CapturePChainAddress("Receiver address", network)
			if err != nil {
				return err
			}
		}
		receiverAddr, err = address.ParseToID(receiverAddrStr)
		if err != nil {
			return err
		}
	} else {
		receiverAddr = kc.Addresses().List()[0]
		receiverAddrStr, err = address.Format("P", key.GetHRP(networkID), receiverAddr[:])
		if err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("this operation is going to:")
	if send {
		addr := kc.Addresses().List()[0]
		addrStr, err := address.Format("P", key.GetHRP(networkID), addr[:])
		if err != nil {
			return err
		}
		if addr == receiverAddr {
			return fmt.Errorf("sender addr is the same as receiver addr")
		}
		ux.Logger.PrintToUser("- send %.9f AVAX from %s to target address %s", float64(amount)/float64(units.Avax), addrStr, receiverAddrStr)
		ux.Logger.PrintToUser("- take a fee of %.9f AVAX from source address %s", float64(4*fee)/float64(units.Avax), addrStr)
	} else {
		ux.Logger.PrintToUser("- receive %.9f AVAX at target address %s", float64(amount)/float64(units.Avax), receiverAddrStr)
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
		Addrs:     []ids.ShortID{receiverAddr},
	}

	if send {
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
		ux.Logger.PrintToUser("Issuing ExportTx P -> X")
		if ledgerIndex != wrongLedgerIndexVal {
			ux.Logger.PrintToUser("*** Please sign 'Export Tx / P to X Chain' transaction on the ledger device *** ")
		}
		if _, err := wallet.P().IssueExportTx(wallet.X().BlockchainID(), outputs); err != nil {
			return err
		}
	} else {
		if receiveRecoveryStep == 0 {
			wallet, err := primary.NewWalletWithTxs(context.Background(), apiEndpoint, kc)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
				return err
			}
			ux.Logger.PrintToUser("Issuing ImportTx P -> X")
			if ledgerIndex != wrongLedgerIndexVal {
				ux.Logger.PrintToUser("*** Please sign ImportTx transaction on the ledger device *** ")
			}
			if _, err = wallet.X().IssueImportTx(avago_constants.PlatformChainID, &to); err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
				return err
			}
			time.Sleep(2 * time.Second)
			receiveRecoveryStep++
		}
		if receiveRecoveryStep == 1 {
			wallet, err := primary.NewWalletWithTxs(context.Background(), apiEndpoint, kc)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
			output := &avax.TransferableOutput{
				Asset: avax.Asset{ID: wallet.P().AVAXAssetID()},
				Out: &secp256k1fx.TransferOutput{
					Amt:          amount + fee*1,
					OutputOwners: to,
				},
			}
			outputs := []*avax.TransferableOutput{output}
			ux.Logger.PrintToUser("Issuing ExportTx X -> P")
			if ledgerIndex != wrongLedgerIndexVal {
				ux.Logger.PrintToUser("*** Please sign 'Export Tx / X to P Chain' transaction on the ledger device *** ")
			}
			if _, err := wallet.X().IssueExportTx(avago_constants.PlatformChainID, outputs); err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
			time.Sleep(2 * time.Second)
			receiveRecoveryStep++
		}
		if receiveRecoveryStep == 2 {
			wallet, err := primary.NewWalletWithTxs(context.Background(), apiEndpoint, kc)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
			ux.Logger.PrintToUser("Issuing ImportTx X -> P")
			if ledgerIndex != wrongLedgerIndexVal {
				ux.Logger.PrintToUser("*** Please sign ImportTx transaction on the ledger device *** ")
			}
			if _, err = wallet.P().IssueImportTx(wallet.X().BlockchainID(), &to); err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
		}
	}

	return nil
}
